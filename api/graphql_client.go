package api

import (
	"crypto/tls"
	"net/http"

	graphql "github.com/hasura/go-graphql-client"
)

type GraphQLClient struct {
	Client *graphql.Client
}

// authTransport injects the Bearer token into every request.
type authTransport struct {
	token string
	base  http.RoundTripper
}

func NewGraphQLClient(c *Client, baseUrl string, skipTls bool) *GraphQLClient {
	var base http.RoundTripper = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: skipTls},
	}
	if c.Debug {
		base = &debugTransport{base: base}
	}
	httpClient := &http.Client{
		Transport: &authTransport{
			token: c.AccessToken,
			base:  base,
		},
	}
  
	gqlClient := graphql.NewClient(c.HubHost+"/hub/graphql", httpClient)

	return &GraphQLClient{Client: gqlClient}
}

func (t *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+t.token)
	return t.base.RoundTrip(req)
}