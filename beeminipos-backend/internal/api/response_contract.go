package api

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
func pathMatches(template, path string) bool {
	templateParts, pathParts := strings.Split(strings.Trim(template, "/"), "/"), strings.Split(strings.Trim(path, "/"), "/")
	if len(templateParts) != len(pathParts) {
		return false
	}
	for index := range templateParts {
		if strings.HasPrefix(templateParts[index], "{") && strings.HasSuffix(templateParts[index], "}") {
			if pathParts[index] == "" {
				return false
			}
			continue
		}
		if templateParts[index] != pathParts[index] {
			return false
		}
	}
	return true
}
func successContracts(method, requestPath string, status int) ([]successResponseContract, bool, bool) {
	path, operationFound := strings.TrimPrefix(requestPath, "/public/v1"), false
	contracts := make([]successResponseContract, 0, 1)
	for _, contract := range generatedSuccessResponses {
		if contract.Method != method || !pathMatches(contract.Path, path) {
			continue
		}
		operationFound = true
		if contract.Status == status {
			contracts = append(contracts, contract)
		}
	}
	return contracts, operationFound, len(contracts) > 0
}
func enforceSuccessResponses(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/public/v1/") {
			next.ServeHTTP(w, r)
			return
		}
		if err := validateOpenAPIRequest(r, strings.TrimPrefix(r.URL.Path, "/public/v1")); err != nil {
			problem(w, http.StatusBadRequest, "REQUEST_CONTRACT_VIOLATION")
			return
		}
		captured := &bufferedResponse{header: make(http.Header)}
		next.ServeHTTP(captured, r)
		if captured.status == 0 {
			captured.status = http.StatusOK
		}
		if captured.status >= 200 && captured.status < 300 {
			contracts, operationFound, statusFound := successContracts(r.Method, r.URL.Path, captured.status)
			valid := operationFound && statusFound
			contentType := strings.TrimSpace(strings.Split(captured.header.Get("Content-Type"), ";")[0])
			var decoded any
			decodedJSON := false
			for _, contract := range contracts {
				if !valid {
					break
				}
				if len(contract.Media) == 0 {
					valid = captured.body.Len() == 0
					continue
				}
				valid = captured.body.Len() > 0 && containsString(contract.Media, contentType)
				if valid && contentType == "application/json" && !responseSchemaIsBinary(contract.Schema) {
					if !decodedJSON {
						valid = json.Unmarshal(captured.body.Bytes(), &decoded) == nil
						decodedJSON = valid
					}
					if valid {
						valid = validateResponseSchema(contract.Schema, decoded) == nil
					}
				}
			}
			if !valid {
				problem(w, http.StatusInternalServerError, "RESPONSE_CONTRACT_VIOLATION")
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
				problem(w, http.StatusInternalServerError, "RESPONSE_CONTRACT_VIOLATION")
				return
			}
		} else {
			problem(w, http.StatusInternalServerError, "RESPONSE_CONTRACT_VIOLATION")
			return
		}
		for key, values := range captured.header {
			w.Header()[key] = append([]string(nil), values...)
		}
		w.WriteHeader(captured.status)
		_, _ = w.Write(captured.body.Bytes())
	})
}

func validateOpenAPIRequest(r *http.Request, path string) error {
	var contract *requestContract
	for index := range generatedRequestContracts {
		candidate := &generatedRequestContracts[index]
		if candidate.Method == r.Method && pathMatches(candidate.Path, path) {
			contract = candidate
			break
		}
	}
	if contract == nil {
		return nil
	}
	if err := validateGeneratedRequestParameters(contract.Parameters, contract.Path, path, r.URL.Query(), r.Header); err != nil {
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
