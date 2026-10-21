package domain

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestBLEServerPersistedResultSemanticCorruptionNeverPublishes(t *testing.T) {
	for _, name := range []string{"metadata device", "metadata name", "metadata pin", "metadata digest", "metadata expiry", "metadata CA", "challenge digest", "invalid PEM", "trailing certificate", "unknown JSON", "wrong key", "clientAuth", "mixed EKU", "wrong DNS", "wrong URI", "wrong CN", "wrong issuer", "wrong leaf expiry", "old leaf start"} {
		t.Run(name, func(t *testing.T) {
			svc, _, store, key := bleLifecycleFixture(t, "plain")
			envelope, e := svc.NewBLEServerCertificateChallenge(bleFixtureDeviceID, "activation", "binding", "tenant")
			if e != nil {
				t.Fatal(e)
			}
			c, proof, sig := bleProof(t, envelope, key)
			certificate, e := svc.IssueBLEServerCertificate(c.ChallengeID, "tenant", proof, sig)
			if e != nil {
				t.Fatal(e)
			}
			repo := svc.repo.(*MemoryRepository)
			record, e := repo.Resource("ble_server_certificate", c.ChallengeID)
			if e != nil {
				t.Fatal(e)
			}
			switch name {
			case "metadata device":
				certificate.DeviceID = "22222222-2222-4222-8222-222222222222"
			case "metadata name":
				certificate.ServerName = "foreign.ble.beefiscal.invalid"
			case "metadata pin":
				certificate.SPKISHA256 = strings.Repeat("0", 64)
			case "metadata digest":
				certificate.CertificateSHA256 = strings.Repeat("0", 64)
			case "metadata expiry":
				certificate.NotAfter = certificate.NotAfter.Add(time.Second)
			case "metadata CA":
				certificate.CAChainPEM += "\ncorrupt"
			case "challenge digest":
				record.Data["challenge_sha256"] = strings.Repeat("0", 64)
			case "invalid PEM":
				certificate.CertificatePEM = "parseable JSON but no certificate"
			case "trailing certificate":
				certificate.CertificatePEM += certificate.CertificatePEM
			default:
				if name != "unknown JSON" {
					block, _ := pem.Decode([]byte(certificate.CertificatePEM))
					leaf, e := x509.ParseCertificate(block.Bytes)
					if e != nil {
						t.Fatal(e)
					}
					public := &key.PublicKey
					ca := svc.bleServerIssuer.ca
					signer := svc.bleServerIssuer.key
					switch name {
					case "wrong key":
						other, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
						public = &other.PublicKey
					case "clientAuth":
						leaf.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
					case "mixed EKU":
						leaf.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}
					case "wrong DNS":
						leaf.DNSNames = []string{"other.ble.beefiscal.invalid"}
					case "wrong URI":
						u, _ := url.Parse("spiffe://beefiscal/ble-server/device/other")
						leaf.URIs = []*url.URL{u}
					case "wrong CN":
						leaf.Subject.CommonName = "other"
					case "wrong issuer":
						_, _, ca, signer = bleProfileCA(t, []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth})
					case "wrong leaf expiry":
						leaf.NotAfter = leaf.NotAfter.Add(time.Second)
					case "old leaf start":
						leaf.NotBefore = time.Unix(c.IssuedAt-3600, 0)
					}
					// Parsed certificates retain extensions: clear them so changed typed fields
					// are actually encoded into the hostile synthetic certificate.
					leaf.ExtraExtensions = nil
					leaf.RawSubject = nil
					der, e := x509.CreateCertificate(rand.Reader, leaf, ca, public, signer)
					if e != nil {
						t.Fatal(e)
					}
					certificate.CertificatePEM = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
					digest := sha256.Sum256(der)
					certificate.CertificateSHA256 = hex.EncodeToString(digest[:])
				}
			}
			raw, _ := json.Marshal(certificate)
			if name == "unknown JSON" {
				raw = []byte(strings.TrimSuffix(string(raw), "}") + `,"other":true}`)
			}
			record.Data["certificate_json"] = string(raw)
			// Test-only corruption injection models structurally parseable storage damage,
			// bypassing immutable production APIs; restart exercises the stored result.
			repo.mu.Lock()
			repo.resources[resourceKey(record.Kind, record.ID)] = record
			e = repo.persistLocked()
			repo.mu.Unlock()
			if e != nil {
				t.Fatal(e)
			}
			restarted, e := NewPersistentRepository(store)
			if e != nil {
				t.Fatal(e)
			}
			svc.repo = restarted
			got, e := svc.IssueBLEServerCertificate(c.ChallengeID, "tenant", proof, sig)
			if e == nil || got.CertificatePEM != "" {
				t.Fatal("semantic corruption published", name, e)
			}
		})
	}
}
func TestBLEServerProofSignatureWireBoundsBeforeCAOrSave(t *testing.T) {
	for _, name := range []string{"oversized", "padded", "pad bits", "short"} {
		t.Run(name, func(t *testing.T) {
			svc, store, _, key := bleLifecycleFixture(t, "plain")
			envelope, e := svc.NewBLEServerCertificateChallenge(bleFixtureDeviceID, "activation", "binding", "tenant")
			if e != nil {
				t.Fatal(e)
			}
			c, proof, sig := bleProof(t, envelope, key)
			switch name {
			case "oversized":
				sig = strings.Repeat("A", 1024*1024)
			case "padded":
				sig += "=="
			case "short":
				sig = sig[:85]
			case "pad bits":
				alphabet := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
				n := strings.IndexByte(alphabet, sig[len(sig)-1])
				sig = sig[:len(sig)-1] + string(alphabet[n+1])
				if _, e = base64.RawURLEncoding.DecodeString(sig); e != nil {
					t.Fatal("counterexample must decode with old decoder", e)
				}
			}
			calls := store.calls
			clockCalls := 0
			svc.bleServerIssuer.clock = func() time.Time { clockCalls++; return time.Unix(c.IssuedAt, 0) }
			got, e := svc.IssueBLEServerCertificate(c.ChallengeID, "tenant", proof, sig)
			if e == nil || got.CertificatePEM != "" {
				t.Fatal("invalid wire signature accepted", e)
			}
			if clockCalls != 0 || store.calls != calls {
				t.Fatal("invalid signature reached CA/time or Save", clockCalls, store.calls-calls)
			}
		})
	}
}
