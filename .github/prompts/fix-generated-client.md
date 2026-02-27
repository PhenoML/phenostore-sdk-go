The OpenAPI spec was updated and `phenostore/gen/generated.go` has been
regenerated. Review the generated code to understand what changed in the API,
then update the hand-written files to match.

## Steps

1. Run `go build ./...` to see if there are compilation errors.
2. Diff or read `phenostore/gen/generated.go` to understand the API changes
   (new endpoints, renamed types, changed parameters, etc.).
3. Update `phenostore/client.go`, `phenostore/errors.go`, and `examples/`
   as needed — both to fix any compilation errors and to expose meaningful
   new API surface through the convenience layer.
4. Run `go build ./...` and `go vet ./...` to verify.

## Guidelines

- Follow the existing wrapper style: thin convenience methods on `Client`
  that delegate to `c.inner`, returning either `json.RawMessage` or a
  typed struct like `gen.Bundle`.
- If a new endpoint fits the existing CRUD pattern (read/create/update/delete
  a FHIR resource), add a convenience method for it.
- If an endpoint is specialized or rarely used, it's fine to leave it
  accessible only via `client.Inner()` — not everything needs a wrapper.
- Update examples if they reference changed APIs or if a new feature
  warrants a usage example.
