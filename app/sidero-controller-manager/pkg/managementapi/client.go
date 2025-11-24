// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package managementapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/pkg/errors"
)

const (
	defaultTimeout     = 30 * time.Second
	defaultAPIVersion  = "v1"
	contentTypeJSON    = "application/json"
)

// Client represents a Management API client.
type Client struct {
	baseURL    string
	httpClient *http.Client
	apiKey     string // Optional API key for authentication
}

// ClientOption is a functional option for configuring the Client.
type ClientOption func(*Client)

// WithTimeout sets the HTTP client timeout.
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *Client) {
		c.httpClient.Timeout = timeout
	}
}

// WithAPIKey sets the API key for authentication.
func WithAPIKey(apiKey string) ClientOption {
	return func(c *Client) {
		c.apiKey = apiKey
	}
}

// NewClient creates a new Management API client.
func NewClient(baseURL string, opts ...ClientOption) (*Client, error) {
	// Validate and normalize base URL
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, errors.Wrap(err, "invalid base URL")
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("invalid URL scheme: %s (must be http or https)", u.Scheme)
	}

	client := &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}

	// Apply options
	for _, opt := range opts {
		opt(client)
	}

	return client, nil
}

// HealthCheck checks if the Management API is reachable and healthy.
func (c *Client) HealthCheck(ctx context.Context) (*HealthCheckResponse, error) {
	var response HealthCheckResponse
	if err := c.doRequest(ctx, http.MethodGet, "/health", nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// RegisterCluster registers a new cluster with the Management API.
func (c *Client) RegisterCluster(ctx context.Context, req *ClusterRegisterRequest) (*ClusterRegisterResponse, error) {
	var response ClusterRegisterResponse
	if err := c.doRequest(ctx, http.MethodPost, fmt.Sprintf("/api/%s/clusters", defaultAPIVersion), req, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// GetCluster retrieves cluster information from the Management API.
func (c *Client) GetCluster(ctx context.Context, clusterID string) (*ClusterInfoResponse, error) {
	var response ClusterInfoResponse
	if err := c.doRequest(ctx, http.MethodGet, fmt.Sprintf("/api/%s/clusters/%s", defaultAPIVersion, clusterID), nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// UpdateClusterStatus updates the status of a cluster.
func (c *Client) UpdateClusterStatus(ctx context.Context, clusterID string, req *StatusUpdateRequest) (*StatusUpdateResponse, error) {
	var response StatusUpdateResponse
	if err := c.doRequest(ctx, http.MethodPatch, fmt.Sprintf("/api/%s/clusters/%s/status", defaultAPIVersion, clusterID), req, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// RegisterNode registers a new node with the Management API.
func (c *Client) RegisterNode(ctx context.Context, req *NodeRegisterRequest) (*NodeRegisterResponse, error) {
	var response NodeRegisterResponse
	if err := c.doRequest(ctx, http.MethodPost, fmt.Sprintf("/api/%s/nodes", defaultAPIVersion), req, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// GetNode retrieves node information from the Management API.
func (c *Client) GetNode(ctx context.Context, nodeID string) (*NodeInfoResponse, error) {
	var response NodeInfoResponse
	if err := c.doRequest(ctx, http.MethodGet, fmt.Sprintf("/api/%s/nodes/%s", defaultAPIVersion, nodeID), nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// UpdateNodeStatus updates the status of a node.
func (c *Client) UpdateNodeStatus(ctx context.Context, nodeID string, req *StatusUpdateRequest) (*StatusUpdateResponse, error) {
	var response StatusUpdateResponse
	if err := c.doRequest(ctx, http.MethodPatch, fmt.Sprintf("/api/%s/nodes/%s/status", defaultAPIVersion, nodeID), req, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// doRequest performs an HTTP request to the Management API.
func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}, response interface{}) error {
	// Build full URL
	fullURL := c.baseURL + path

	// Prepare request body if provided
	var bodyReader io.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return errors.Wrap(err, "failed to marshal request body")
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		return errors.Wrap(err, "failed to create HTTP request")
	}

	// Set headers
	req.Header.Set("Content-Type", contentTypeJSON)
	req.Header.Set("Accept", contentTypeJSON)
	req.Header.Set("User-Agent", "Sidero-ManagementAPI-Client/1.0")

	// Add API key if configured
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "failed to execute HTTP request")
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return errors.Wrap(err, "failed to read response body")
	}

	// Check for non-2xx status codes
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errResp ErrorResponse
		if err := json.Unmarshal(respBody, &errResp); err != nil {
			// Failed to parse error response, return generic error
			return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(respBody))
		}
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, errResp.Detail)
	}

	// Parse response if a response object was provided
	if response != nil {
		if err := json.Unmarshal(respBody, response); err != nil {
			return errors.Wrap(err, "failed to unmarshal response")
		}
	}

	return nil
}

// Close closes the HTTP client (placeholder for any cleanup).
func (c *Client) Close() error {
	// Currently nothing to close, but keeping for future extensibility
	return nil
}
