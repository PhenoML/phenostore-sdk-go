# phenostore-sdk-go

Go SDK for [Phenostore](https://phenoml.com), a FHIR-compatible data store by Phenoml.

## Installation

Requires Go 1.26.0 or later.

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
and Go requirements do not become SDK requirements. The SDK requires Go 1.26.0;
generation uses the toolchain required by `tools/go.mod` (Go can download it
automatically). The `make check-generated` target verifies that the committed
spec, generator version, and generated client agree.

## Automated SDK regeneration

The **Regenerate SDK** workflow accepts upstream `spec-updated` dispatches and
manual runs. It starts from `main`, validates and imports the supplied spec for
upstream dispatches, regenerates the client, and checks the build, race tests,
lint, and generation consistency through `make check`. Runtime checks use the
SDK's minimum Go version; generation uses the separate tools module's toolchain.

If compatibility checks fail, Claude may adapt the handwritten wrapper, tests,
examples, and documentation. All checks must then pass before publication.
Only the imported spec, generated client, and validated handwritten changes are
included; tooling, dependencies, and workflows are excluded.

Runs are serialized and maintain one PR from `sdk/regenerate`. Later runs update
that PR; an upstream dispatch whose SDK already matches `main` closes an obsolete
regeneration PR without creating an empty commit. Manual runs use the committed
spec and leave any pending upstream PR open when there are no changes.
Claude proposes a conventional commit and PR title for actual changes, with a
fixed `feat:` fallback if classification fails. Review the title's release
classification before merging.

PRs created with `GITHUB_TOKEN` do not automatically trigger PR workflows. Run
the **CI** workflow manually on `sdk/regenerate` before merging:

```sh
gh workflow run ci.yml --ref sdk/regenerate
```

Both Claude steps use Anthropic workload identity federation. The federation
rule must allow this repository on `refs/heads/main` for both
`repository_dispatch` and `workflow_dispatch`. Re-running an old workflow run
retains its original workflow commit; trigger a new run after workflow updates.
Enable debug logging on a run to include Claude's full output.

The publishing and payload-validation regression tests run in CI and can be
run locally with Python 3, without installing packages:

```sh
python3 -B -m unittest discover -s .github/scripts/tests -v
```

## Dependency maintenance

Dependabot groups monthly updates to SDK runtime dependencies and GitHub Actions
separately. Routine Go updates target direct dependencies; transitive dependencies
follow their parents. Security updates remain enabled for both SDK and tooling
dependencies, including indirect dependencies, in separate security groups.

The **Update SDK generator** workflow runs monthly or manually. It calls
`make update-generator` to upgrade the generator, regenerate the client from the
committed spec, tidy dependencies, and run `make check` before opening one
maintenance PR. An open maintenance PR from this repository targeting `main`
prevents duplicates. Build, race tests, and lint run on the SDK's minimum Go
version, with generation checks using the tools module's toolchain. The workflow
uses `GITHUB_TOKEN`; run **CI** manually on the maintenance branch before merging,
since these PRs do not automatically trigger PR workflows.

To upgrade and validate the generator locally, run `make update-generator` for
the latest release or `make update-generator GENERATOR_VERSION=v2.7.1` for a
specific version.

To validate other SDK changes, including generator security PRs:

```sh
make check
```

Commit `tools/go.mod`, `tools/go.sum`, any SDK dependency changes, and the
regenerated client together. Review changes to the generated public API and
behavior before merging. The HTTP contract tests exercise the wrapper and
generated client together against a local TLS server, including authentication,
FHIR request serialization, search parameters, bundles, and errors.

CI builds and runs tests on Go 1.26.0 before checking regeneration with the tools
toolchain. Dependency updates that require a newer SDK Go version fail that
check; raising the minimum requires an explicit CI and documentation update.
`make update-generator` also preserves the SDK's minimum Go version and runs
the full `make check` validation locally before returning success. An incompatible
generator update fails validation without opening a maintenance PR; the
automation does not adapt the wrapper to hide a generator compatibility failure.

The generator is currently upgraded to v2.7.1. The public `gen.BearerAuthScopes`
constant now has a named context-key type to prevent collisions with string keys.
This is a breaking change: use `string(gen.BearerAuthScopes)` where a string is
required, and use the typed constant directly when storing or retrieving context
values. A regression test verifies that the typed key is distinct from a string
key with the same value.

Versions v2.8.0 and v2.8.1 fail the HTTP contract tests: they parse the
`Last-Modified` response header as an RFC3339 timestamp, while Phenostore returns
an HTTP date. The upstream OpenAPI
header schema must be corrected and synced here before that upgrade can pass.
Do not bypass the test or edit the imported spec to unblock it. Until then,
maintenance runs targeting `latest` will fail validation without opening a PR.

## License

[MIT](LICENSE)
