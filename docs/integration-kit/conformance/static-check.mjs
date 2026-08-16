import { readFileSync } from "node:fs";
const spec=readFileSync(new URL("../openapi.yaml",import.meta.url),"utf8");
for(const required of ["/integration/v1/enrollments","/integration/v1/organization","/integration/v1/locations/{source_id}","/integration/v1/registers/{source_id}","/integration/v1/operators/{source_id}","/integration/v1/credentials:rotate","/integration/v1/credentials:revoke","webhooks:","additionalProperties: false"]){if(!spec.includes(required))throw new Error(`missing contract element: ${required}`)}
const ids=[...spec.matchAll(/operationId:\s*([A-Za-z0-9_]+)/g)].map(x=>x[1]);if(new Set(ids).size!==ids.length)throw new Error("duplicate operationId");console.log("OpenAPI static contract checks passed");
