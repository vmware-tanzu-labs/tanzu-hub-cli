package cmd

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

// ensure credentials are passed in and assign env vars if used
func validateCredentials(cmd *cobra.Command) {
	userOk := validateVarEnv(&username, "HUB_USERNAME")
	passOk := validateVarEnv(&password, "HUB_PASSWORD")
	url_ok := validateVarEnv(&hub_url, "HUB_TARGET")

	if strings.EqualFold(os.Getenv("HUB_SKIP_TLS"), "true") {
		skipTls = true
	}
	
	if !userOk || !passOk {
		fmt.Fprintln(os.Stderr, "Credentials not provided!")
		fmt.Fprintln(os.Stderr, "You must either provide the username and password as arguements")
		fmt.Fprintf(os.Stderr, "or you must export them as HUB_USERNAME and HUB_PASSWORD environment variables.\n\n")
		_ = cmd.Usage()
		os.Exit(1)
	}
	if !url_ok {
		fmt.Fprintln(os.Stderr, "Hub target not provided!")
		fmt.Fprintln(os.Stderr, "You must either provide the FQDN as arguement")
		fmt.Fprintf(os.Stderr, "or you must export it as HUB_TARGET environment variable.\n\n")
		_ = cmd.Usage()
		os.Exit(1)
	}

	hub_url = strings.TrimRight(hub_url, "/")
	if !strings.HasPrefix(hub_url, "https://") {
		hub_url = "https://" + strings.TrimPrefix(hub_url, "http://")
	}
}

var htmlTagRe = regexp.MustCompile(`<[^>]+>`)

func stripHTML(s string) string {
	return strings.TrimSpace(htmlTagRe.ReplaceAllString(s, ""))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + ".."
}

// Check if param is set and if not retrieve env var if set
func validateVarEnv(param *string, key string) bool {
	if *param == "" {
		if value, ok := os.LookupEnv(key); ok {
			*param = value
		} else {
			return false
		}
	}
	return true
}