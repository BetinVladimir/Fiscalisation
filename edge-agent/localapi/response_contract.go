package localapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

type successResponseContract struct {
	Method, Path, Operation string
	Status                  int
	Media                   []string
	Schema                  string
}
type requestContract struct {
	Method, Path, Operation string
	Required                bool
	Media                   []string
	Schema                  string
	Parameters              string
}
type bufferedResponse struct {
	header http.Header
	body   bytes.Buffer
	status int
}

func (w *bufferedResponse) Header() http.Header { return w.header }
func (w *bufferedResponse) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}
func (w *bufferedResponse) Write(value []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.body.Write(value)
}
func edgePathMatches(template, path string) bool {
	a, b := strings.Split(strings.Trim(template, "/"), "/"), strings.Split(strings.Trim(path, "/"), "/")
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if strings.HasPrefix(a[i], "{") && strings.HasSuffix(a[i], "}") {
			if b[i] == "" {
				return false
			}
			continue
		}
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
func enforceEdgeSuccess(w http.ResponseWriter, r *http.Request, next func(http.ResponseWriter, *http.Request)) {
	if err := validateEdgeOpenAPIRequest(r); err != nil {
		edgeProblem(w, http.StatusUnprocessableEntity, "REQUEST_CONTRACT_VIOLATION", "request violates OpenAPI contract")
		return
	}
	captured := &bufferedResponse{header: make(http.Header)}
	next(captured, r)
	if captured.status == 0 {
		captured.status = http.StatusOK
	}
	if captured.status >= 200 && captured.status < 300 {
		matched, valid := false, false
		for _, c := range generatedSuccessResponses {
			if c.Method == r.Method && edgePathMatches(c.Path, r.URL.Path) {
				matched = true
				if c.Status == captured.status {
					contentType := strings.TrimSpace(strings.Split(captured.header.Get("Content-Type"), ";")[0])
					valid = len(c.Media) > 0 && captured.body.Len() > 0 && c.Media[0] == contentType
					if valid && contentType == "application/json" && !responseSchemaIsBinary(c.Schema) {
						var decoded any
						valid = json.Unmarshal(captured.body.Bytes(), &decoded) == nil && validateResponseSchema(c.Schema, decoded) == nil
					}
				}
			}
		}
		if !matched || !valid {
			http.Error(w, "response contract violation", http.StatusInternalServerError)
			return
		}
	} else if captured.status >= 400 {
		contentType := strings.TrimSpace(strings.Split(captured.header.Get("Content-Type"), ";")[0])
		var decoded any
		valid := contentType == "application/problem+json" && json.Unmarshal(captured.body.Bytes(), &decoded) == nil && validateResponseSchema(generatedProblemResponseSchema, decoded) == nil
		if object, ok := decoded.(map[string]any); !ok || object["status"] != float64(captured.status) {
			valid = false
		}
		if !valid {
			edgeProblem(w, http.StatusInternalServerError, "RESPONSE_CONTRACT_VIOLATION", "response contract violation")
			return
		}
	} else {
		edgeProblem(w, http.StatusInternalServerError, "RESPONSE_CONTRACT_VIOLATION", "response contract violation")
		return
	}
	for key, values := range captured.header {
		w.Header()[key] = append([]string(nil), values...)
	}
	w.WriteHeader(captured.status)
	_, _ = w.Write(captured.body.Bytes())
}

func validateEdgeOpenAPIRequest(r *http.Request) error {
	var contract *requestContract
	for index := range generatedRequestContracts {
		candidate := &generatedRequestContracts[index]
		if candidate.Method == r.Method && edgePathMatches(candidate.Path, r.URL.Path) {
			contract = candidate
			break
		}
	}
	if contract == nil {
		return nil
	}
	if err := validateGeneratedRequestParameters(contract.Parameters, contract.Path, r.URL.Path, r.URL.Query(), r.Header); err != nil {
		return err
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, (2<<20)+1))
	if err != nil || len(body) > 2<<20 {
		return errors.New("request body unreadable or too large")
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	if len(contract.Media) == 0 {
		if len(bytes.TrimSpace(body)) != 0 {
			return errors.New("request body is not documented")
		}
		return nil
	}
	if len(bytes.TrimSpace(body)) == 0 {
		if contract.Required {
			return errors.New("request body is required")
		}
		return nil
	}
	contentType := strings.TrimSpace(strings.Split(r.Header.Get("Content-Type"), ";")[0])
	if !containsString(contract.Media, contentType) {
		return errors.New("request media type is not documented")
	}
	if contentType == "application/json" {
		var decoded any
		if json.Unmarshal(body, &decoded) != nil || validateResponseSchema(contract.Schema, decoded) != nil {
			return errors.New("request body violates schema")
		}
	}
	return nil
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
