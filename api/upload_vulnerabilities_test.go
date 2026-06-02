package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUploadVulnerabilities(t *testing.T) {
	// Create a temporary file to upload
	tmpFile, err := os.CreateTemp("", "test-upload-*.zip")
	require.NoError(t, err, "Failed to create temp file")
	defer func() {
		if err := os.Remove(tmpFile.Name()); err != nil {
			t.Logf("Failed to remove temp file: %v", err)
		}
	}()

	content := []byte("dummy zip content")
	_, err = tmpFile.Write(content)
	require.NoError(t, err, "Failed to write to temp file")
	require.NoError(t, tmpFile.Close(), "Failed to close temp file")

	// Mock server to simulate the upload endpoint
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle access token endpoint
		if r.URL.Path == TokenEndpoint {
			mockAuthEndpoint(w)
			return
		}
		if r.URL.Path == uploadEndpoint {
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "Bearer mock-token-123", r.Header.Get("Authorization"))
			assert.Equal(t, "SECURITY_METADATA", r.URL.Query().Get("category"))
			w.WriteHeader(http.StatusAccepted)
		}
	}))
	defer server.Close()

	// Test the upload function
	client, err := GetAccessToken("user", "pass", server.URL, true)
	require.NoError(t, err, "Failed to get access token")

	err = client.UploadVulnerabilities(tmpFile.Name(), "vulnerability")
	require.NoError(t, err)
}
