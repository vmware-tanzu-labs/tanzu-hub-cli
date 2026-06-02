package api

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const TokenEndpoint = "/csp/gateway/am/api/auth/token"

type Client struct {
	HttpClient  *http.Client
	AccessToken string
	HubHost     string
	Debug       bool
}

func GetAccessToken(username, password, url string, skipTls bool, debug ...bool) (*Client, error) {
	payload := fmt.Sprintf("client_id=tp_app&grant_type=password&username=%s&password=%s",
		username, password)

	var base http.RoundTripper = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: skipTls},
	}
	dbg := len(debug) > 0 && debug[0]
	if dbg {
		base = &debugTransport{base: base}
	}
	httpClient := &http.Client{
		Transport: base,
		Timeout:   time.Second * 5,
	}

	resp, err := httpClient.Post(url+TokenEndpoint, "application/x-www-form-urlencoded", bytes.NewBuffer([]byte(payload)))
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result struct {
		AccessToken string `json:"access_token"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result.AccessToken == "" {
		return nil, fmt.Errorf("access_token not found in response")
	}

	client := &Client{
		HttpClient:  httpClient,
		AccessToken: result.AccessToken,
		HubHost:     url,
		Debug:       dbg,
	}

	return client, nil
}
