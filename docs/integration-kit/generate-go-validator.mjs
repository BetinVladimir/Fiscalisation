import { readFileSync, writeFileSync } from "node:fs";
import { resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import YAML from "yaml";

const here=dirname(fileURLToPath(import.meta.url));
const spec=YAML.parse(readFileSync(resolve(here,"openapi.yaml"),"utf8"));
const names={organization:"OrganizationResource",location:"LocationResource",register:"RegisterResource",operator:"OperatorResource"};
const dereference=value=>{
  if(Array.isArray(value)) return value.map(dereference);
  if(!value||typeof value!=="object") return value;
  if(value.$ref){const parts=value.$ref.replace(/^#\//,"").split("/");let target=spec;for(const part of parts)target=target[part];return dereference(target)}
  return Object.fromEntries(Object.entries(value).map(([key,nested])=>[key,dereference(nested)]));
};
const schemas=Object.fromEntries(Object.entries(names).map(([kind,name])=>[kind,JSON.stringify({$schema:"http://json-schema.org/draft-07/schema#",...dereference(spec.components.schemas[name])})]));
const quoted=value=>JSON.stringify(value);
const entries=Object.entries(schemas).map(([kind,schema])=>`\t${quoted(kind)}: ${quoted(schema)},`).join("\n");
const output=`// Code generated from docs/integration-kit/openapi.yaml; DO NOT EDIT.\npackage integration\n\nimport (\n  "errors"\n  "fmt"\n\n  "github.com/xeipuuv/gojsonschema"\n)\n\nvar generatedResourceSchemas = map[string]string{\n${entries}\n}\n\nfunc validateResourcePayload(kind string, raw []byte) error {\n  schema, ok := generatedResourceSchemas[kind]\n  if !ok { return errors.New("unsupported resource type") }\n  result, err := gojsonschema.Validate(gojsonschema.NewStringLoader(schema), gojsonschema.NewBytesLoader(raw))\n  if err != nil { return errors.New("resource body must be a JSON object") }\n  if result.Valid() { return nil }\n  first := result.Errors()[0]\n  return fmt.Errorf("resource schema violation at %s: %s", first.Field(), first.Description())\n}\n`;
writeFileSync(resolve(here,"../../fiscal-backend/internal/integration/resource_schemas_gen.go"),output);
