// Opt-in test host: real integration credential authentication + production
// domain issuer. Requires private generated fixture files and a disposable DB.
package main

import (
	"encoding/json"
	"errors"
	"fiscalisation/fiscal-backend/internal/domain"
	"fiscalisation/fiscal-backend/internal/integration"
	"fiscalisation/fiscal-backend/internal/mqttclient"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
)

type fileStore struct {
	mu   sync.Mutex
	path string
}

func (s *fileStore) Load() ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return os.ReadFile(s.path)
}
func (s *fileStore) Save(b []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := os.WriteFile(s.path+".next", b, 0600); e != nil {
		return e
	}
	return os.Rename(s.path+".next", s.path)
}
func main() {
	if len(os.Args) != 3 {
		log.Fatal("usage: execution-pipeline-oracle PRIVATE_FIXTURE_DIR POSTGRES_URL")
	}
	dir, db := os.Args[1], os.Args[2]
	if dir == "" || db == "" {
		log.Fatal(errors.New("explicit disposable setup required"))
	}
	repo, e := domain.NewPersistentRepository(&fileStore{path: filepath.Join(dir, "root.snapshot.json")})
	must(e)
	svc := domain.NewService(repo, mqttclient.NewBridge("01234567890123456789012345678901", repo))
	ca, e := os.ReadFile(filepath.Join(dir, "root-ca.pem"))
	must(e)
	key, e := os.ReadFile(filepath.Join(dir, "root-key.pem"))
	must(e)
	issuer, e := domain.NewX509DeviceCredentialIssuer(ca, key, "ssl://mqtt.test:8883", "wss://mqtt.test/mqtt", "01234567890123456789012345678901")
	must(e)
	svc.SetDeviceCredentialIssuer(issuer)
	svc.SetBLESigningKey("test-only-readiness-secret-32bytes")
	authority, e := integration.New(db, []byte("abcdef0123456789abcdef0123456789"), []byte("0123456789abcdef0123456789abcdef"))
	must(e)
	defer authority.Close()
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	must(e)
	body, _ := json.Marshal(map[string]string{"url": "http://" + listener.Addr().String()})
	must(os.WriteFile(filepath.Join(dir, "root-server.json"), body, 0600))
	log.Fatal(http.Serve(listener, &integration.HTTPHandler{Service: authority, ExecutionAuthorizations: svc}))
}
func must(e error) {
	if e != nil {
		log.Fatal(e)
	}
}
