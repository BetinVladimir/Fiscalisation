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
	templateParts := strings.Split(strings.Trim(template, "/"), "/")
	pathParts := strings.Split(strings.Trim(path, "/"), "/")
	if len(templateParts) != len(pathParts) {
		return false
	}
	for index := range templateParts {
		templatePart := templateParts[index]
		open, close := strings.IndexByte(templatePart, '{'), strings.IndexByte(templatePart, '}')
		if open >= 0 && close > open {
			prefix, suffix := templatePart[:open], templatePart[close+1:]
			actual := pathParts[index]
			if !strings.HasPrefix(actual, prefix) || !strings.HasSuffix(actual, suffix) || len(actual) <= len(prefix)+len(suffix) {
				return false
			}
			continue
		}
		if templatePart != pathParts[index] {
			return false
		}
	}
	return true
}

func pathSpecificity(template string) int {
	score, parameter := 0, false
	for _, character := range template {
		if character == '{' {
			parameter = true
			continue
		}
		if character == '}' {
			parameter = false
			continue
		}
		if !parameter {
			score++
		}
	}
	return score
}

func successContract(method, requestPath string, status int) (successResponseContract, bool, bool) {
	path := strings.TrimPrefix(requestPath, "/public/v1")
	operationFound := false
	bestSpecificity := -1
	var best successResponseContract
	bestStatus := false
	for _, contract := range generatedSuccessResponses {
		if contract.Method != method || !pathMatches(contract.Path, path) {
			continue
		}
		operationFound = true
		specificity := pathSpecificity(contract.Path)
		if specificity > bestSpecificity {
			bestSpecificity, best, bestStatus = specificity, contract, contract.Status == status
		}
	}
	return best, operationFound, bestStatus
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
			contract, operationFound, statusFound := successContract(r.Method, r.URL.Path, captured.status)
			valid := operationFound && statusFound
			contentType := strings.TrimSpace(strings.Split(captured.header.Get("Content-Type"), ";")[0])
			if valid && len(contract.Media) == 0 {
				valid = captured.body.Len() == 0
			} else if valid {
				valid = captured.body.Len() > 0 && containsString(contract.Media, contentType)
				if valid && contentType == "application/json" && !responseSchemaIsBinary(contract.Schema) {
					var decoded any
					valid = json.Unmarshal(captured.body.Bytes(), &decoded) == nil && validateResponseSchema(contract.Schema, decoded) == nil
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
	bestSpecificity := -1
	for index := range generatedRequestContracts {
		candidate := &generatedRequestContracts[index]
		if candidate.Method == r.Method && pathMatches(candidate.Path, path) {
			specificity := pathSpecificity(candidate.Path)
			if specificity > bestSpecificity {
				contract, bestSpecificity = candidate, specificity
			}
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
