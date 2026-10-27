# phenostore-sdk-go

Go SDK for [Phenostore](https://phenoml.com), a FHIR-compatible data store by Phenoml.

## Installation

```sh
go get github.com/phenoml/phenostore-sdk-go
```

## Usage

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/phenoml/phenostore-sdk-go/phenostore"
)

func main() {
	client, err := phenostore.NewClient(
		"https://api.phenostore.example.com",
		"your-client-id",
		"your-client-secret",
		"my-tenant",
		"my-store",
	)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	// Read a Patient
	patient, err := client.ReadResource(ctx, "Patient", "pat-123")
	if phenostore.IsNotFound(err) {
		fmt.Println("not found")
		return
	}
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(patient))

	// Create an Observation
	obs := json.RawMessage(`{
		"resourceType": "Observation",
		"status": "final",
		"code": {"text": "Blood pressure"},
		"subject": {"reference": "Patient/pat-123"}
	}`)
	created, err := client.CreateResource(ctx, "Observation", obs, nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(created))
}
```

## Convenience methods

| Method | Description |
|--------|-------------|
| `ReadResource` | Get a resource by type and ID |
| `CreateResource` | Create a new resource |
| `UpdateResource` | Update (or upsert) a resource |
| `DeleteResource` | Delete a resource |
| `SearchResources` | Search with FHIR query parameters |
| `ProcessBundle` | Execute a transaction or batch bundle |

For operations not covered by convenience methods (patch, history, bulk, etc.), use `client.Inner()` to access the full generated API client.

## Authentication

The client authenticates via OAuth2 client credentials. Tokens are acquired lazily and refreshed automatically.

```go
// Custom HTTP client (e.g. for custom TLS)
client, err := phenostore.NewClient(baseURL, clientID, clientSecret, tenant, store,
	phenostore.WithHTTPClient(myHTTPClient),
)
```

Use `phenostore.WithScopes(...)` to request specific OAuth2 scopes during token exchange.

## Code generation

The client code in `phenostore/gen/` is generated from the OpenAPI spec in `api/openapi.yaml` using [oapi-codegen](https://github.com/oapi-codegen/oapi-codegen). The spec is imported from the upstream Phenostore API repo — do not edit it directly here. To regenerate after a spec update:

```sh
make generate
```

The generator is pinned in the separate `tools/go.mod` module so its dependencies
and Go requirements do not become SDK requirements. The SDK supports Go 1.24.0;
generation uses the toolchain required by `tools/go.mod` (Go can download it
automatically). The `make check-generated` target verifies that the committed
spec, generator version, and generated client agree.

## Dependency maintenance

Dependabot groups monthly updates to SDK runtime dependencies and GitHub Actions
separately. Routine Go updates target direct dependencies; transitive dependencies
follow their parents. Security updates remain enabled for both SDK and tooling
dependencies, including indirect dependencies, in separate security groups.

The **Update SDK generator** workflow runs monthly or manually. It upgrades the
generator, regenerates the client from the committed spec, tidies dependencies,
and runs build, race tests, and generation checks before opening one maintenance
PR. An open maintenance PR prevents another from being created. It uses
`GITHUB_TOKEN`; approve pending PR workflows in GitHub to run the normal CI
checks on the SDK's minimum Go version and run lint before merging.

To prepare the same change locally, or finish a generator security PR:

```sh
make update-generator                         # latest generator
make update-generator GENERATOR_VERSION=v2.6.0  # a specific version
go build ./...
make test check-generated lint
```

Commit `tools/go.mod`, `tools/go.sum`, any SDK dependency changes, and the
regenerated client together. Review changes to the generated public API and
behavior before merging. The HTTP contract tests exercise the wrapper and
generated client together against a local TLS server, including authentication,
FHIR request serialization, search parameters, bundles, and errors.

CI builds and runs tests on Go 1.24.0 before checking regeneration with the tools
toolchain. Dependency updates that require a newer SDK Go version fail that
check; raising the minimum requires an explicit CI and documentation update.
`make update-generator` also preserves the SDK's minimum Go version.

The generator is currently upgraded to v2.6.0. Version v2.7.2 changes the public
`gen.BearerAuthScopes` constant to a named context-key type, breaking existing
assignments to `string`; a compile-time compatibility check guards that usage.
Versions v2.8.0 and v2.8.1 fail the HTTP contract tests: they parse the
`Last-Modified` response header as an RFC3339 timestamp, while Phenostore returns
an HTTP date. The upstream OpenAPI
header schema must be corrected and synced here before that upgrade can pass.
Do not bypass the test or edit the imported spec to unblock it. Until then,
maintenance runs targeting `latest` will fail validation without opening a PR.

## License

[MIT](LICENSE)
