package gen

import "encoding/json"

// RawJSON is an alias for json.RawMessage, matching the x-go-type
// annotation in the OpenAPI spec for FHIR resource fields stored as raw JSON.
type RawJSON = json.RawMessage
