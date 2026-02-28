package phenostore

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/phenoml/phenostore-sdk-go/phenostore/gen"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// Client wraps the generated ClientWithResponses with OAuth2 authentication
// and convenience methods for common FHIR operations.
type Client struct {
	inner  *gen.ClientWithResponses
	tenant string
	store  string
}

// Option configures the Phenostore client.
type Option func(*options)

type options struct {
	scopes     []string
	httpClient *http.Client
}

// WithScopes sets the OAuth2 scopes to request.
func WithScopes(scopes ...string) Option {
	return func(o *options) {
		o.scopes = scopes
	}
}

// WithHTTPClient sets the base HTTP client used for OAuth2 token exchange.
// Use this for custom TLS configuration or testing.
func WithHTTPClient(hc *http.Client) Option {
	return func(o *options) {
		o.httpClient = hc
	}
}

// NewClient creates a Phenostore client that authenticates using OAuth2
// client credentials. Tokens are acquired lazily and refreshed automatically.
func NewClient(baseURL, clientID, clientSecret, tenant, store string, opts ...Option) (*Client, error) {
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}

	ccConfig := &clientcredentials.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		TokenURL:     baseURL + "/oauth/token",
		Scopes:       o.scopes,
		AuthStyle:    oauth2.AuthStyleInParams,
	}

	ctx := context.Background()
	if o.httpClient != nil {
		ctx = context.WithValue(ctx, oauth2.HTTPClient, o.httpClient)
	}

	httpClient := ccConfig.Client(ctx)

	inner, err := gen.NewClientWithResponses(baseURL, gen.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("creating client: %w", err)
	}

	return &Client{inner: inner, tenant: tenant, store: store}, nil
}

// Tenant returns the tenant configured on this client.
func (c *Client) Tenant() string {
	return c.tenant
}

// Store returns the store configured on this client.
func (c *Client) Store() string {
	return c.store
}

// Inner returns the generated ClientWithResponses for operations not covered
// by the convenience methods (patch, history, bulk, docref, validate).
func (c *Client) Inner() *gen.ClientWithResponses {
	return c.inner
}

// ReadResource retrieves a single FHIR resource by type and ID.
func (c *Client) ReadResource(ctx context.Context, resourceType, id string) (json.RawMessage, error) {
	resp, err := c.inner.ReadResourceWithResponse(ctx, c.tenant, c.store, gen.ResourceType(resourceType), id)
	if err != nil {
		return nil, err
	}
	if resp.HTTPResponse.StatusCode >= 400 {
		return nil, &OperationOutcomeError{StatusCode: resp.HTTPResponse.StatusCode, Body: resp.Body}
	}
	return resp.Body, nil
}

// CreateResource creates a new FHIR resource. Returns the created resource.
// Handles both 200 (conditional create, already exists) and 201 (created).
func (c *Client) CreateResource(ctx context.Context, resourceType string, body json.RawMessage, params *gen.CreateResourceParams) (json.RawMessage, error) {
	resp, err := c.inner.CreateResourceWithResponse(ctx, c.tenant, c.store, gen.ResourceType(resourceType), params, body)
	if err != nil {
		return nil, err
	}
	if resp.HTTPResponse.StatusCode >= 400 {
		return nil, &OperationOutcomeError{StatusCode: resp.HTTPResponse.StatusCode, Body: resp.Body}
	}
	return resp.Body, nil
}

// UpdateResource updates an existing FHIR resource, or creates it (upsert).
// Handles both 200 (updated) and 201 (created via upsert).
func (c *Client) UpdateResource(ctx context.Context, resourceType, id string, body json.RawMessage, params *gen.UpdateResourceParams) (json.RawMessage, error) {
	resp, err := c.inner.UpdateResourceWithResponse(ctx, c.tenant, c.store, gen.ResourceType(resourceType), id, params, body)
	if err != nil {
		return nil, err
	}
	if resp.HTTPResponse.StatusCode >= 400 {
		return nil, &OperationOutcomeError{StatusCode: resp.HTTPResponse.StatusCode, Body: resp.Body}
	}
	return resp.Body, nil
}

// DeleteResource deletes a FHIR resource.
func (c *Client) DeleteResource(ctx context.Context, resourceType, id string) error {
	resp, err := c.inner.DeleteResourceWithResponse(ctx, c.tenant, c.store, gen.ResourceType(resourceType), id)
	if err != nil {
		return err
	}
	if resp.HTTPResponse.StatusCode >= 400 {
		return &OperationOutcomeError{StatusCode: resp.HTTPResponse.StatusCode, Body: resp.Body}
	}
	return nil
}

// SearchResources searches for FHIR resources matching the given parameters.
func (c *Client) SearchResources(ctx context.Context, resourceType string, params *gen.SearchResourcesParams) (*gen.Bundle, error) {
	resp, err := c.inner.SearchResourcesWithResponse(ctx, c.tenant, c.store, gen.ResourceType(resourceType), params)
	if err != nil {
		return nil, err
	}
	if resp.HTTPResponse.StatusCode >= 400 {
		return nil, &OperationOutcomeError{StatusCode: resp.HTTPResponse.StatusCode, Body: resp.Body}
	}
	var bundle gen.Bundle
	if err := json.Unmarshal(resp.Body, &bundle); err != nil {
		return nil, fmt.Errorf("unmarshaling bundle: %w", err)
	}
	return &bundle, nil
}

// ProcessBundle processes a FHIR transaction or batch bundle.
func (c *Client) ProcessBundle(ctx context.Context, bundle json.RawMessage) (*gen.Bundle, error) {
	resp, err := c.inner.ProcessBundleWithResponse(ctx, c.tenant, c.store, bundle)
	if err != nil {
		return nil, err
	}
	if resp.HTTPResponse.StatusCode >= 400 {
		return nil, &OperationOutcomeError{StatusCode: resp.HTTPResponse.StatusCode, Body: resp.Body}
	}
	var result gen.Bundle
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("unmarshaling bundle: %w", err)
	}
	return &result, nil
}
