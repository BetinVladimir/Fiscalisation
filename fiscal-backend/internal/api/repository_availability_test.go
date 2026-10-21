package api

import (
	"context"
	"errors"
	"fiscalisation/fiscal-backend/internal/domain"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type availabilityStore struct {
	data   []byte
	failed bool
}

func (s *availabilityStore) Load() ([]byte, error) {
	if s.failed {
		return nil, errors.New("reload outage")
	}
	return append([]byte(nil), s.data...), nil
}
func (s *availabilityStore) Save(b []byte) error {
	if s.failed {
		return errors.New("save uncertain")
	}
	s.data = append([]byte(nil), b...)
	return nil
}
func TestRepositoryAvailabilityMapsOutageAndConcurrentFailure(t *testing.T) {
	for _, during := range []bool{false, true} {
		t.Run(map[bool]string{false: "before request", true: "during request"}[during], func(t *testing.T) {
			store := &availabilityStore{}
			repo, e := domain.NewPersistentRepository(store)
			if e != nil {
				t.Fatal(e)
			}
			svc := domain.NewService(repo, domain.NewSimulator(true))
			fail := func() {
				store.failed = true
				if e := repo.PutResource(domain.ResourceRecord{Kind: "issued_binding", ID: "binding", TenantID: "tenant", Version: 1, Data: map[string]any{"signature": "synthetic"}}); e == nil {
					t.Fatal("injected failure missing")
				}
			}
			called := false
			handler := repositoryAvailability(svc, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				if during {
					fail()
				}
				problem(w, 404, "NOT_FOUND")
			}))
			if !during {
				fail()
			}
			out := httptest.NewRecorder()
			handler.ServeHTTP(out, httptest.NewRequest("GET", "/public/v1/resources/binding", nil))
			if out.Code != 503 || !strings.Contains(out.Body.String(), `"retryable":true`) || strings.Contains(out.Body.String(), "NOT_FOUND") {
				t.Fatal(out.Code, out.Body.String())
			}
			if called != during {
				t.Fatal("preflight did not stop handler")
			}
			if out.Header().Get("Retry-After") != "1" {
				t.Fatal("retry header missing")
			}
			store.failed = false
			refreshed := httptest.NewRecorder()
			handler.ServeHTTP(refreshed, httptest.NewRequest("GET", "/", nil))
			if refreshed.Code != 503 {
				t.Fatal("refresh must require retry", refreshed.Code)
			}
		})
	}
}

func TestRepositoryAvailabilityDoesNotReturnEmptySuccessAfterConcurrentOutage(t *testing.T) {
	store := &availabilityStore{}
	repo, e := domain.NewPersistentRepository(store)
	if e != nil {
		t.Fatal(e)
	}
	svc := domain.NewService(repo, domain.NewSimulator(true))
	handler := repositoryAvailability(svc, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		store.failed = true
		_ = repo.PutResource(domain.ResourceRecord{Kind: "issued_binding", ID: "binding", TenantID: "tenant", Version: 1, Data: map[string]any{"signature": "synthetic"}})
		if got := repo.Resources("issued_binding", "tenant"); len(got) != 0 {
			t.Fatal("authority leaked")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`[]`))
	}))
	out := httptest.NewRecorder()
	handler.ServeHTTP(out, httptest.NewRequest("GET", "/resources", nil))
	if out.Code != 503 || strings.TrimSpace(out.Body.String()) == "[]" {
		t.Fatal("false successful absence", out.Code, out.Body.String())
	}
}

func TestRepositoryAvailabilityRejectsObservationFromEarlierRecoveryEpoch(t *testing.T) {
	for _, status := range []int{200, 404} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			store := &availabilityStore{}
			repo, e := domain.NewPersistentRepository(store)
			if e != nil {
				t.Fatal(e)
			}
			svc := domain.NewService(repo, domain.NewSimulator(true))
			handler := repositoryAvailability(svc, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				store.failed = true
				_ = repo.PutResource(domain.ResourceRecord{Kind: "issued_binding", ID: "binding", TenantID: "tenant", Version: 1, Data: map[string]any{"signature": "synthetic"}})
				if _, e := repo.Resource("issued_binding", "binding"); !errors.Is(e, domain.ErrRepositoryUnavailable) {
					t.Fatal("expected unavailable observation", e)
				}
				// Another caller completes recovery before this handler formats its old result.
				store.failed = false
				if e := repo.PersistenceReady(); !errors.Is(e, domain.ErrRepositoryUnavailable) {
					t.Fatal("refresh expected", e)
				}
				if e := repo.PersistenceReady(); e != nil {
					t.Fatal("recovery did not finish", e)
				}
				if status == 404 {
					problem(w, 404, "NOT_FOUND")
				} else {
					w.WriteHeader(200)
					_, _ = w.Write([]byte(`[]`))
				}
			}))
			out := httptest.NewRecorder()
			handler.ServeHTTP(out, httptest.NewRequest("GET", "/resources", nil))
			if out.Code != 503 {
				t.Fatal("stale observation escaped after recovery", out.Code, out.Body.String())
			}
		})
	}
}

func TestRepositoryAvailabilityMiddlewareOwnerDoesNotFinalizeStaleReplay(t *testing.T) {
	for _, empty := range []bool{false, true} {
		t.Run(map[bool]string{false: "saveReplay", true: "saveEmptyReplay"}[empty], func(t *testing.T) {
			store := &availabilityStore{}
			repo, e := domain.NewPersistentRepository(store)
			if e != nil {
				t.Fatal(e)
			}
			svc := domain.NewService(repo, domain.NewSimulator(true))
			h := &Handler{svc: svc}
			executions := 0
			router := http.NewServeMux()
			router.HandleFunc("POST /public/v1/epoch-test", func(w http.ResponseWriter, r *http.Request) {
				executions++
				// Exercise an actual domain mutation before the failing observation.
				if e := repo.PutResource(domain.ResourceRecord{Kind: "epoch_test", ID: "mutation", TenantID: "tenant", Version: 1, Data: map[string]any{"status": "COMPLETED"}}); e != nil {
					t.Fatal(e)
				}
				store.failed = true
				_ = repo.PutResource(domain.ResourceRecord{Kind: "epoch_test", ID: "failed", TenantID: "tenant", Version: 1, Data: map[string]any{}})
				if _, e := repo.Resource("epoch_test", "failed"); !errors.Is(e, domain.ErrRepositoryUnavailable) {
					t.Fatal(e)
				}
				store.failed = false
				_ = repo.PersistenceReady()
				if e := repo.PersistenceReady(); e != nil {
					t.Fatal(e)
				}
				w.Header().Set("Etag", `"stale"`)
				w.Header().Set("Location", "/false-entity")
				w.Header().Set("Content-Length", "123")
				w.Header().Set("Content-Encoding", "gzip")
				if empty {
					h.saveEmptyReplay(w, r, []byte(`{}`), 204)
				} else {
					h.saveReplay(w, r, []byte(`{}`), 404, map[string]any{"code": "FALSE_NOT_FOUND"})
				}
			})
			handler := repositoryAvailability(svc, fiscalIdempotency(svc, router))
			request := func(body, key string) *httptest.ResponseRecorder {
				r := httptest.NewRequest("POST", "/public/v1/epoch-test", strings.NewReader(body))
				r.Header.Set("Idempotency-Key", key)
				out := httptest.NewRecorder()
				handler.ServeHTTP(out, r)
				return out
			}
			key := "epoch-idempotency-key-0001"
			first := request(`{}`, key)
			if first.Code != 503 {
				t.Fatal("stale result escaped", first.Code, first.Body.String())
			}
			for _, header := range []string{"Etag", "Location", "Content-Length", "Content-Encoding"} {
				if first.Header().Get(header) != "" {
					t.Fatal("stale entity metadata retained", header, first.Header())
				}
			}
			replay, ok := repo.Replay(" POST /public/v1/epoch-test " + key)
			if !ok || !replay.Pending || replay.Status != 503 {
				t.Fatal("stale result finalized", ok, replay)
			}
			second := request(`{}`, key)
			if second.Code != 503 || executions != 1 {
				t.Fatal("ambiguous command re-executed", second.Code, executions)
			}
			mismatch := request(`{"changed":true}`, key)
			if mismatch.Code != 409 || executions != 1 {
				t.Fatal("mismatched retry re-executed", mismatch.Code, executions)
			}
			invalid := request(`{}`, "short")
			if invalid.Code != 400 || executions != 1 {
				t.Fatal("invalid key dispatched", invalid.Code, executions)
			}
			restarted, e := domain.NewPersistentRepository(store)
			if e != nil {
				t.Fatal(e)
			}
			replay, ok = restarted.Replay(" POST /public/v1/epoch-test " + key)
			if !ok || !replay.Pending {
				t.Fatal("restart lost pending ambiguity", ok, replay)
			}
		})
	}
}
func TestRepositoryAvailabilityMiddlewareOwnerOrdinarySuccessReplays(t *testing.T) {
	for _, empty := range []bool{false, true} {
		t.Run(map[bool]string{false: "body", true: "empty"}[empty], func(t *testing.T) {
			store := &availabilityStore{}
			repo, _ := domain.NewPersistentRepository(store)
			svc := domain.NewService(repo, domain.NewSimulator(true))
			h := &Handler{svc: svc}
			calls := 0
			handler := repositoryAvailability(svc, fiscalIdempotency(svc, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if empty {
					h.saveEmptyReplay(w, r, []byte(`{}`), 204)
				} else {
					h.saveReplay(w, r, []byte(`{}`), 201, map[string]any{"id": "committed"})
				}
			})))
			var first *httptest.ResponseRecorder
			for i := 0; i < 2; i++ {
				r := httptest.NewRequest("POST", "/public/v1/ordinary", strings.NewReader(`{}`))
				r.Header.Set("Idempotency-Key", "ordinary-idempotency-key")
				out := httptest.NewRecorder()
				handler.ServeHTTP(out, r)
				if i == 0 {
					first = out
				} else if out.Code != first.Code || out.Body.String() != first.Body.String() || out.Header().Get("Idempotency-Replayed") != "true" {
					t.Fatal("ordinary replay changed", out.Code, out.Body.String())
				}
			}
			if calls != 1 {
				t.Fatal("ordinary command reexecuted", calls)
			}
		})
	}
}

func TestRepositoryAvailabilityStandaloneReplayHelpersStillPersist(t *testing.T) {
	for _, empty := range []bool{false, true} {
		t.Run(map[bool]string{false: "body", true: "empty"}[empty], func(t *testing.T) {
			store := &availabilityStore{}
			repo, _ := domain.NewPersistentRepository(store)
			svc := domain.NewService(repo, domain.NewSimulator(true))
			h := &Handler{svc: svc}
			r := httptest.NewRequest("POST", "/public/v1/standalone", strings.NewReader(`{}`))
			r.Header.Set("Idempotency-Key", "standalone-idempotency-key")
			out := httptest.NewRecorder()
			if empty {
				h.saveEmptyReplay(out, r, []byte(`{}`), 204)
			} else {
				h.saveReplay(out, r, []byte(`{}`), 201, map[string]any{"id": "stored"})
			}
			replay, ok := repo.Replay(replayKey(r))
			if !ok || replay.Pending || replay.Status != out.Code || string(replay.Body) != out.Body.String() {
				t.Fatal("standalone persistence changed", ok, replay, out.Code, out.Body.String())
			}
		})
	}
}

func TestRepositoryAvailabilityRejectsEarlierEpochBeforeRouterAdmission(t *testing.T) {
	store := &availabilityStore{}
	repo, e := domain.NewPersistentRepository(store)
	if e != nil {
		t.Fatal(e)
	}
	svc := domain.NewService(repo, domain.NewSimulator(true))
	earlier := svc.PersistenceEpoch()
	store.failed = true
	_ = repo.PutResource(domain.ResourceRecord{Kind: "epoch_test", ID: "failed", TenantID: "tenant", Version: 1, Data: map[string]any{}})
	store.failed = false
	_ = repo.PersistenceReady()
	if e := repo.PersistenceReady(); e != nil {
		t.Fatal(e)
	}
	if earlier == svc.PersistenceEpoch() {
		t.Fatal("actual recovery did not change epoch")
	}
	called := false
	handler := repositoryAvailability(svc, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true; w.WriteHeader(200) }))
	request := httptest.NewRequest("POST", "/public/v1/financial-command", strings.NewReader(`{}`))
	request = request.WithContext(context.WithValue(request.Context(), repositoryEpochKey{}, earlier))
	out := httptest.NewRecorder()
	handler.ServeHTTP(out, request)
	if called || out.Code != 503 {
		t.Fatal("old request admitted after recovery", called, out.Code)
	}
}

type versionedAvailabilityStore struct {
	availabilityStore
	generation int64
}

func (s *versionedAvailabilityStore) LoadVersioned() ([]byte, int64, error) {
	b, e := s.Load()
	return b, s.generation, e
}
func (s *versionedAvailabilityStore) SaveVersioned(b []byte, expected int64) (int64, error) {
	if expected != s.generation {
		return s.generation, errors.New("generation conflict")
	}
	e := s.Save(b)
	if e == nil {
		s.generation++
	}
	return s.generation, e
}
func TestRepositoryAvailabilityHealthyRemoteGenerationRefreshAdmitsNewCommand(t *testing.T) {
	store := &versionedAvailabilityStore{}
	local, e := domain.NewPersistentRepository(store)
	if e != nil {
		t.Fatal(e)
	}
	remote, e := domain.NewPersistentRepository(store)
	if e != nil {
		t.Fatal(e)
	}
	if e = remote.PutResource(domain.ResourceRecord{Kind: "test", ID: "remote", TenantID: "tenant", Version: 1, Data: map[string]any{"status": "COMPLETED"}}); e != nil {
		t.Fatal(e)
	}
	svc := domain.NewService(local, domain.NewSimulator(true))
	h := &Handler{svc: svc}
	calls := 0
	handler := repositoryAvailability(svc, fiscalIdempotency(svc, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		h.saveReplay(w, r, []byte(`{}`), 201, map[string]any{"id": "new-command"})
	})))
	for i := 0; i < 2; i++ {
		request := httptest.NewRequest("POST", "/public/v1/normal-remote-refresh", strings.NewReader(`{}`))
		request.Header.Set("Idempotency-Key", "healthy-remote-generation-key")
		out := httptest.NewRecorder()
		handler.ServeHTTP(out, request)
		if out.Code != 201 {
			t.Fatal("healthy generation refresh treated as outage", out.Code, out.Body.String())
		}
		if i == 1 && out.Header().Get("Idempotency-Replayed") != "true" {
			t.Fatal("normal replay lost")
		}
	}
	if calls != 1 {
		t.Fatal("healthy command did not execute exactly once", calls)
	}
	replay, ok := local.Replay(" POST /public/v1/normal-remote-refresh healthy-remote-generation-key")
	if !ok || replay.Pending || replay.Status != 201 {
		t.Fatal("healthy command marked ambiguous", ok, replay)
	}
	if _, e = local.Resource("test", "remote"); e != nil {
		t.Fatal("healthy remote state not adopted", e)
	}
}
