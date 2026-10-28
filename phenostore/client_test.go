package phenostore_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/phenoml/phenostore-sdk-go/phenostore"
	"github.com/phenoml/phenostore-sdk-go/phenostore/gen"
)

const resourcePath = "/v1/tenants/test-tenant/stores/test-store/Patient"
const patientJSON = `{"resourceType":"Patient","id":"pat-123","active":true,"extension":[{"url":"https://example.com/custom","valueString":"preserved"}]}`

func TestBearerAuthScopeContextKey(t *testing.T) {
	scopes := []string{"system/*.read"}
	ctx := context.WithValue(context.Background(), gen.BearerAuthScopes, scopes)
	if got := ctx.Value(gen.BearerAuthScopes); !reflect.DeepEqual(got, scopes) {
		t.Errorf("scopes = %v, want %v", got, scopes)
	}
	if got := ctx.Value(string(gen.BearerAuthScopes)); got != nil {
		t.Errorf("typed scope key collided with a string key: %v", got)
	}
}

// TLS also verifies that WithHTTPClient reaches both the OAuth exchange and
// the generated API client. No credentials or live Phenostore service are needed.
func newTestClient(t *testing.T, api http.HandlerFunc) (*phenostore.Client, *atomic.Int32) {
	t.Helper()
	tokens := new(atomic.Int32)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth/token" {
			tokens.Add(1)
			if r.Method != http.MethodPost {
				t.Errorf("token method = %s, want POST", r.Method)
			}
			if err := r.ParseForm(); err != nil {
				t.Errorf("parsing token request: %v", err)
			}
			want := url.Values{
				"grant_type":    {"client_credentials"},
				"client_id":     {"test-client"},
				"client_secret": {"test-secret"},
				"scope":         {"system/*.read system/*.write"},
			}
			if !reflect.DeepEqual(r.PostForm, want) {
				t.Errorf("token form = %v, want %v", r.PostForm, want)
			}
			if r.Header.Get("Authorization") != "" {
				t.Error("credentials should be sent as form parameters")
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"access_token":"test-token","token_type":"Bearer","expires_in":3600}`)
			return
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization = %q, want Bearer test-token", got)
		}
		api(w, r)
	}))
	t.Cleanup(server.Close)
	client, err := phenostore.NewClient(server.URL, "test-client", "test-secret", "test-tenant", "test-store",
		phenostore.WithHTTPClient(server.Client()),
		phenostore.WithScopes("system/*.read", "system/*.write"))
	if err != nil {
		t.Fatal(err)
	}
	if tokens.Load() != 0 {
		t.Fatal("NewClient acquired a token before the first API request")
	}
	return client, tokens
}

func checkJSONBody(t *testing.T, r *http.Request, want string) {
	t.Helper()
	if got := r.Header.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
	var gotJSON, wantJSON any
	if err := json.NewDecoder(r.Body).Decode(&gotJSON); err != nil {
		t.Errorf("decoding request: %v", err)
		return
	}
	if err := json.Unmarshal([]byte(want), &wantJSON); err != nil {
		t.Errorf("decoding expected JSON: %v", err)
		return
	}
	if !reflect.DeepEqual(gotJSON, wantJSON) {
		t.Errorf("request JSON = %v, want %v", gotJSON, wantJSON)
	}
}

func TestResourceRequests(t *testing.T) {
	condition := "identifier=https://example.com/mrn|123"
	etag := `W/"2"`
	read := func(c *phenostore.Client) (json.RawMessage, error) {
		return c.ReadResource(context.Background(), "Patient", "pat-123")
	}
	create := func(c *phenostore.Client) (json.RawMessage, error) {
		return c.CreateResource(context.Background(), "Patient", json.RawMessage(patientJSON), &gen.CreateResourceParams{IfNoneExist: &condition})
	}
	update := func(c *phenostore.Client) (json.RawMessage, error) {
		return c.UpdateResource(context.Background(), "Patient", "pat-123", json.RawMessage(patientJSON), &gen.UpdateResourceParams{IfMatch: &etag})
	}
	tests := []struct {
		name   string
		method string
		path   string
		status int
		header string
		value  string
		call   func(*phenostore.Client) (json.RawMessage, error)
	}{
		{"read", "GET", resourcePath + "/pat-123", 200, "", "", read},
		{"create", "POST", resourcePath, 201, "If-None-Exist", condition, create},
		{"conditional create", "POST", resourcePath, 200, "If-None-Exist", condition, create},
		{"update", "PUT", resourcePath + "/pat-123", 200, "If-Match", etag, update},
		{"upsert", "PUT", resourcePath + "/pat-123", 201, "If-Match", etag, update},
		{"delete", "DELETE", resourcePath + "/pat-123", 204, "", "", func(c *phenostore.Client) (json.RawMessage, error) {
			return nil, c.DeleteResource(context.Background(), "Patient", "pat-123")
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var requests atomic.Int32
			client, tokens := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				if r.Method != tt.method || r.URL.Path != tt.path || r.URL.RawQuery != "" {
					t.Errorf("request = %s %s, want %s %s", r.Method, r.URL, tt.method, tt.path)
				}
				if tt.header != "" && r.Header.Get(tt.header) != tt.value {
					t.Errorf("%s = %q, want %q", tt.header, r.Header.Get(tt.header), tt.value)
				}
				if tt.method == "POST" || tt.method == "PUT" {
					checkJSONBody(t, r, patientJSON)
				}
				w.Header().Set("Content-Type", "application/fhir+json")
				w.Header().Set("ETag", etag)
				w.Header().Set("Last-Modified", "Tue, 27 Oct 2026 09:00:00 GMT")
				w.WriteHeader(tt.status)
				if tt.status != http.StatusNoContent {
					_, _ = io.WriteString(w, patientJSON)
				}
			})
			// A second request must reuse the cached token.
			for range 2 {
				body, err := tt.call(client)
				if err != nil {
					t.Fatal(err)
				}
				if tt.status != http.StatusNoContent && string(body) != patientJSON {
					t.Errorf("response = %s, want %s", body, patientJSON)
				}
			}
			if requests.Load() != 2 || tokens.Load() != 1 {
				t.Errorf("API requests = %d, token requests = %d; want 2 and 1", requests.Load(), tokens.Load())
			}
		})
	}
}

func TestSearchResources(t *testing.T) {
	count, cursor := 25, "next+/=&?"
	include := []string{"Patient:organization", "Patient:general-practitioner"}
	response := `{"resourceType":"Bundle","type":"searchset","total":1,"entry":[{"resource":` + patientJSON + `}],"link":[{"relation":"next","url":"https://example.com/next"}]}`
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != resourcePath {
			t.Errorf("request = %s %s, want GET %s", r.Method, r.URL, resourcePath)
		}
		want := url.Values{"_count": {"25"}, "_cursor": {cursor}, "_include": include}
		if !reflect.DeepEqual(r.URL.Query(), want) {
			t.Errorf("query = %v, want %v", r.URL.Query(), want)
		}
		w.Header().Set("Content-Type", "application/fhir+json")
		_, _ = io.WriteString(w, response)
	})
	bundle, err := client.SearchResources(context.Background(), "Patient", &gen.SearchResourcesParams{
		UnderscoreCount: &count, UnderscoreCursor: &cursor, UnderscoreInclude: &include,
	})
	if err != nil {
		t.Fatal(err)
	}
	if bundle.ResourceType != "Bundle" || bundle.Type != "searchset" || bundle.Total == nil || *bundle.Total != 1 {
		t.Fatalf("unexpected search bundle: %+v", bundle)
	}
	if bundle.Entry == nil || len(*bundle.Entry) != 1 || (*bundle.Entry)[0].Resource == nil || string(*(*bundle.Entry)[0].Resource) != patientJSON {
		t.Fatalf("resource JSON was not preserved: %+v", bundle.Entry)
	}
	if bundle.Link == nil || len(*bundle.Link) != 1 || (*bundle.Link)[0].Relation != "next" {
		t.Fatalf("pagination links were not decoded: %+v", bundle.Link)
	}
}

func TestProcessBundle(t *testing.T) {
	request := `{"resourceType":"Bundle","type":"transaction","entry":[{"request":{"method":"POST","url":"Patient"},"resource":` + patientJSON + `}]}`
	response := `{"resourceType":"Bundle","type":"transaction-response","entry":[{"response":{"status":"201 Created","location":"Patient/pat-123"}}]}`
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/v1/tenants/test-tenant/stores/test-store" {
			t.Errorf("unexpected bundle request: %s %s", r.Method, r.URL)
		}
		checkJSONBody(t, r, request)
		w.Header().Set("Content-Type", "application/fhir+json")
		_, _ = io.WriteString(w, response)
	})
	bundle, err := client.ProcessBundle(context.Background(), json.RawMessage(request))
	if err != nil {
		t.Fatal(err)
	}
	if bundle.Type != "transaction-response" || bundle.Entry == nil || len(*bundle.Entry) != 1 {
		t.Fatalf("unexpected transaction response: %+v", bundle)
	}
	entry := (*bundle.Entry)[0].Response
	if entry == nil || entry.Status == nil || *entry.Status != "201 Created" || entry.Location == nil || *entry.Location != "Patient/pat-123" {
		t.Fatalf("unexpected entry response: %+v", entry)
	}
}

func TestOperationOutcomeErrors(t *testing.T) {
	operations := map[string]func(*phenostore.Client) error{
		"read": func(c *phenostore.Client) error {
			_, err := c.ReadResource(context.Background(), "Patient", "missing")
			return err
		},
		"create": func(c *phenostore.Client) error {
			_, err := c.CreateResource(context.Background(), "Patient", json.RawMessage(patientJSON), nil)
			return err
		},
		"update": func(c *phenostore.Client) error {
			_, err := c.UpdateResource(context.Background(), "Patient", "missing", json.RawMessage(patientJSON), nil)
			return err
		},
		"delete": func(c *phenostore.Client) error {
			return c.DeleteResource(context.Background(), "Patient", "missing")
		},
		"search": func(c *phenostore.Client) error {
			_, err := c.SearchResources(context.Background(), "Patient", nil)
			return err
		},
		"bundle": func(c *phenostore.Client) error {
			_, err := c.ProcessBundle(context.Background(), json.RawMessage(`{"resourceType":"Bundle","type":"transaction"}`))
			return err
		},
	}
	for name, operation := range operations {
		t.Run(name, func(t *testing.T) {
			for _, status := range []int{403, 404, 410, 412, 500} {
				t.Run(fmt.Sprint(status), func(t *testing.T) {
					outcome := `{"resourceType":"OperationOutcome","issue":[{"severity":"error","code":"processing","diagnostics":"request rejected"}]}`
					client, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
						w.Header().Set("Content-Type", "application/fhir+json")
						w.WriteHeader(status)
						_, _ = io.WriteString(w, outcome)
					})
					err := operation(client)
					var got *phenostore.OperationOutcomeError
					if !errors.As(err, &got) || got.StatusCode != status || string(got.Body) != outcome {
						t.Fatalf("error = %v, want OperationOutcomeError with HTTP %d and raw body", err, status)
					}
					if got.Error() != "[error/processing] request rejected" {
						t.Errorf("unexpected error message: %s", got.Error())
					}
					wrapped := fmt.Errorf("API request: %w", err)
					if phenostore.IsForbidden(wrapped) != (status == 403) ||
						phenostore.IsNotFound(wrapped) != (status == 404) ||
						phenostore.IsGone(wrapped) != (status == 410) ||
						phenostore.IsConflict(wrapped) != (status == 412) {
						t.Errorf("error classification disagrees with HTTP %d", status)
					}
				})
			}
		})
	}
}

func TestInvalidBundleResponse(t *testing.T) {
	for _, operation := range []string{"search", "bundle"} {
		t.Run(operation, func(t *testing.T) {
			client, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/fhir+json")
				_, _ = io.WriteString(w, `{"resourceType":`)
			})
			var err error
			if operation == "search" {
				_, err = client.SearchResources(context.Background(), "Patient", nil)
			} else {
				_, err = client.ProcessBundle(context.Background(), json.RawMessage(`{}`))
			}
			if err == nil {
				t.Fatal("invalid response JSON was accepted")
			}
		})
	}
}
