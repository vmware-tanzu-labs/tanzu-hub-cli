package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mockAuthEndpoint(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"access_token": "mock-token-123", "expires_in": 3600}`))
}

func TestGetAccessToken(t *testing.T) {
	// Mock server to simulate Tanzu Hub token endpoint
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mockAuthEndpoint(w)
	}))
	defer server.Close()

	// Assuming GetAccessToken takes a URL or we can configure the client
	// This is a representative test structure for the api package
	client, err := GetAccessToken("user", "pass", server.URL, true)
	require.NoError(t, err)
	assert.Equal(t, "mock-token-123", client.AccessToken)
}
