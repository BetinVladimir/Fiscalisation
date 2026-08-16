// Code generated from docs/integration-kit/openapi.yaml; DO NOT EDIT.
package integration

import (
  "errors"
  "fmt"

  "github.com/xeipuuv/gojsonschema"
)

var generatedResourceSchemas = map[string]string{
	"organization": "{\"$schema\":\"http://json-schema.org/draft-07/schema#\",\"type\":\"object\",\"additionalProperties\":false,\"required\":[\"legal_name\",\"tax_identifier\"],\"properties\":{\"legal_name\":{\"type\":\"string\",\"minLength\":1},\"tax_identifier\":{\"type\":\"object\",\"additionalProperties\":false,\"required\":[\"country\",\"type\",\"value\"],\"properties\":{\"country\":{\"type\":\"string\",\"pattern\":\"^[A-Z]{2}$\"},\"type\":{\"type\":\"string\",\"pattern\":\"^[A-Z][A-Z0-9_-]{1,62}$\"},\"value\":{\"type\":\"string\",\"minLength\":3,\"maxLength\":128}}},\"address\":{\"type\":\"string\"},\"status\":{\"enum\":[\"ACTIVE\",\"INACTIVE\"]}}}",
	"location": "{\"$schema\":\"http://json-schema.org/draft-07/schema#\",\"type\":\"object\",\"additionalProperties\":false,\"required\":[\"name\",\"address\"],\"properties\":{\"code\":{\"type\":\"string\"},\"name\":{\"type\":\"string\",\"minLength\":1},\"address\":{\"type\":\"string\",\"minLength\":1},\"status\":{\"enum\":[\"ACTIVE\",\"INACTIVE\"]}}}",
	"register": "{\"$schema\":\"http://json-schema.org/draft-07/schema#\",\"type\":\"object\",\"additionalProperties\":false,\"required\":[\"location_source_id\",\"name\"],\"properties\":{\"code\":{\"type\":\"string\"},\"name\":{\"type\":\"string\",\"minLength\":1},\"location_source_id\":{\"type\":\"string\",\"minLength\":1},\"status\":{\"enum\":[\"ACTIVE\",\"BLOCKED\",\"INACTIVE\"]}}}",
	"operator": "{\"$schema\":\"http://json-schema.org/draft-07/schema#\",\"type\":\"object\",\"additionalProperties\":false,\"required\":[\"operator_code\",\"first_name\",\"last_name\",\"roles\"],\"properties\":{\"operator_code\":{\"type\":\"string\",\"pattern\":\"^[A-Za-z0-9]{4}$\"},\"first_name\":{\"type\":\"string\",\"minLength\":1},\"last_name\":{\"type\":\"string\",\"minLength\":1},\"roles\":{\"type\":\"array\",\"minItems\":1,\"items\":{\"enum\":[\"CASHIER\",\"SUPERVISOR\",\"ADMIN\",\"AUDITOR\",\"SERVICE\"]}},\"status\":{\"enum\":[\"ACTIVE\",\"INACTIVE\"]},\"active_from\":{\"type\":\"string\",\"format\":\"date-time\"}}}",
}

func validateResourcePayload(kind string, raw []byte) error {
  schema, ok := generatedResourceSchemas[kind]
  if !ok { return errors.New("unsupported resource type") }
  result, err := gojsonschema.Validate(gojsonschema.NewStringLoader(schema), gojsonschema.NewBytesLoader(raw))
  if err != nil { return errors.New("resource body must be a JSON object") }
  if result.Valid() { return nil }
  first := result.Errors()[0]
  return fmt.Errorf("resource schema violation at %s: %s", first.Field(), first.Description())
}
