# Native preparation and revocation regression

2026-10-21: npm test71/71 PASS; npx tsc --noEmit exit0. Uses actual manager and authority with fake source/OS provider; no root signature or hardware claim. Covers durable-key replay after lost preparation response, identity isolation before monetary preparation,403 durable revocation and in-flight response fencing.
