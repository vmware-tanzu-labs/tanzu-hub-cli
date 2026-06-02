package api

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"os"
)

// debugTransport wraps an http.RoundTripper and logs request and response
// details when debug mode is enabled.
type debugTransport struct {
	base http.RoundTripper
}

func (t *debugTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	reqDump, err := httputil.DumpRequestOut(req, true)
	if err != nil {
		log.Printf("[DEBUG] failed to dump request: %v", err)
	} else {
		log.Printf("[DEBUG] HTTP Request:\n%s", reqDump)
	}

	resp, rtErr := t.base.RoundTrip(req)
	if rtErr != nil {
		log.Printf("[DEBUG] HTTP Error: %v", rtErr)
		return resp, rtErr
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[DEBUG] failed to read response body: %v", err)
		return resp, rtErr
	}
	resp.Body = io.NopCloser(bytes.NewReader(body))

	log.Printf("[DEBUG] HTTP Response: %s\n%s", resp.Status, body)
	fmt.Fprintln(os.Stderr)

	return resp, nil
}
