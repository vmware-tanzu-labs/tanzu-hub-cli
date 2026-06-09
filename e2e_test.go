//go:build integration

package main

import (
	"context"
	"log"
	"os"
	"testing"

	"github.com/laidbackware/tanzu-hub-cli/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type login struct {
	username string
	password string
	fqdn     string
	skipTls  bool
}

func setupSuite() login {
	log.Println("setup suite")
	username, _ := os.LookupEnv("HUB_USER")
	password, _ := os.LookupEnv("HUB_PASS")
	fqdn, _ := os.LookupEnv("HUB_FQDN")
	return login{
		username: username,
		password: password,
		fqdn:     fqdn,
		skipTls:  true,
	}
}

func TestGetAccessToken(t *testing.T) {
	l := setupSuite()

	client, err := api.GetAccessToken(l.username, l.password, l.fqdn, l.skipTls)
	require.NoError(t, err)
	assert.NotNil(t, client)
}

func TestUploadVulnerabilities(t *testing.T) {
	l := setupSuite()

	token, err := api.GetAccessToken(l.username, l.password, l.fqdn, l.skipTls)
	require.NoError(t, err)
	assert.NotNil(t, token)
}

func TestAttachFoundation(t *testing.T) {
	l := setupSuite()

	c, err := api.GetAccessToken(l.username, l.password, l.fqdn, l.skipTls)
	require.NoError(t, err)

	q := api.NewGraphQLClient(c, l.fqdn, l.skipTls)

	results, err := q.AttachSelfManagedCollector(context.Background(), "cheese", "TAS", "managementEndpointID")
	require.NoError(t, err)
	assert.NotNil(t, results)

	queryResults, err := q.QueryManagementEndpoints(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, queryResults)

	err = q.DeleteFoundationByName(context.Background(), "cheese")
	require.NoError(t, err)
}