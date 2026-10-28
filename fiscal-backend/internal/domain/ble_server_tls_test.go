package domain

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"net"
	"testing"
	"time"
)

func TestBLEServerProfileIndependentGoTLSClient(t *testing.T) {
	mqtt, _ := testDeviceCA(t)
	caPEM, keyPEM, ca, caKey := bleProfileCA(t, []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth})
	issuer, e := NewBLEServerCertificateIssuer(caPEM, keyPEM, mqtt)
	if e != nil {
		t.Fatal(e)
	}
	device, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	id := bleFixtureDeviceID
	issued, e := issuer.issue(bleServerIssuancePermit{deviceID: id, public: &device.PublicKey, notAfter: time.Now().Add(time.Hour)})
	if e != nil {
		t.Fatal(e)
	}
	block, _ := pem.Decode([]byte(issued.CertificatePEM))
	original, e := x509.ParseCertificate(block.Bytes)
	if e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"positive", "clientAuth-only", "wrong name", "wrong issuer", "wrong enrolled key"} {
		t.Run(name, func(t *testing.T) {
			leaf := *original
			leafKey := device
			signer := caKey
			parent := ca
			serverName := issued.ServerName
			switch name {
			case "clientAuth-only":
				leaf.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
			case "wrong name":
				serverName = "foreign.ble.beefiscal.invalid"
			case "wrong issuer":
				_, _, parent, signer = bleProfileCA(t, []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth})
			case "wrong enrolled key":
				leafKey, _ = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
			}
			raw, e := x509.CreateCertificate(rand.Reader, &leaf, parent, &leafKey.PublicKey, signer)
			if e != nil {
				t.Fatal(e)
			}
			cert := tls.Certificate{Certificate: [][]byte{raw}, PrivateKey: leafKey}
			left, right := net.Pipe()
			defer left.Close()
			defer right.Close()
			_ = left.SetDeadline(time.Now().Add(5 * time.Second))
			_ = right.SetDeadline(time.Now().Add(5 * time.Second))
			roots := x509.NewCertPool()
			roots.AddCert(ca)
			server := tls.Server(left, &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) { return &cert, nil }})
			client := tls.Client(right, &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, RootCAs: roots, ServerName: serverName, VerifyConnection: func(state tls.ConnectionState) error {
				if len(state.PeerCertificates) != 1 {
					return errors.New("unexpected chain")
				}
				pin := sha256.Sum256(state.PeerCertificates[0].RawSubjectPublicKeyInfo)
				if hex.EncodeToString(pin[:]) != issued.SPKISHA256 {
					return errors.New("enrolled key mismatch")
				}
				return nil
			}})
			done := make(chan error, 1)
			go func() { e := server.Handshake(); left.Close(); done <- e }()
			clientError := client.Handshake()
			right.Close()
			serverError := <-done
			if name == "positive" {
				if clientError != nil || serverError != nil {
					t.Fatal("produced profile handshake failed", clientError, serverError)
				}
			} else if clientError == nil {
				t.Fatal("independent client accepted forbidden certificate", name)
			}
		})
	}
}
