package cmd

import (
	"fmt"
	"os"
	"github.com/spf13/cobra"
	"github.com/laidbackware/tanzu-hub-cli/api"

)

var AccessTokenCmd = &cobra.Command{
	Use:   "access-token",
	Short: "Generate an access token",
	Run: func(cmd *cobra.Command, args []string) {
		validateCredentials(cmd)
		client, err := api.GetAccessToken(username, password, hub_url, skipTls, debug)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Exited with error: %s\n", err)
			os.Exit(1)
		}
		fmt.Println(client.AccessToken)
	},
}

func init() {
	rootCmd.AddCommand(AccessTokenCmd)
}