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
	)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	// Read a Patient
	patient, err := client.ReadResource(ctx, "my-tenant", "my-store", "Patient", "pat-123")
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
	created, err := client.CreateResource(ctx, "my-tenant", "my-store", "Observation", obs, nil)
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
client, err := phenostore.NewClient(baseURL, clientID, clientSecret,
	phenostore.WithHTTPClient(myHTTPClient),
)
```

Use `phenostore.WithScopes(...)` to request specific OAuth2 scopes during token exchange.

## License

[MIT](LICENSE)
