# Agent Guidelines

## Do not edit

- `phenostore/gen/generated.go` — auto-generated; never edit by hand.
- `api/openapi.yaml` — imported from the upstream Phenostore API repo. Changes must be made upstream; a CI workflow handles regeneration automatically.

## Project structure

- `phenostore/gen/` — generated client and models (do not edit)
- `phenostore/gen/types.go` — hand-written type aliases used by the generated code (e.g. `RawJSON`)
- `phenostore/client.go` — convenience wrapper with OAuth2 auth
- `phenostore/errors.go` — error types and helpers
- `api/openapi.yaml` — OpenAPI spec (imported, do not edit)
- `oapi-codegen.yaml` — code generation config
- `examples/` — usage examples
