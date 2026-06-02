package cmd

import (
	"github.com/spf13/cobra"
)
var(
	username string
	password string
	hub_url string
	skipTls bool
	debug bool
)


var rootCmd = &cobra.Command{
	Use:   "th",
	Version: "0.0.1",
	Short: "Tanzu Hub CLI",
	Long:  `Unofficial CLI for Tanzu Hub `,
}

func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&username, "user", "u", "", "Username used to authenticate [$HUB_USERNAME]")
	rootCmd.PersistentFlags().StringVarP(&password, "pass", "p", "", "Password used to authenticate [$HUB_PASSWORD]")
	rootCmd.PersistentFlags().StringVarP(&hub_url, "fqdn", "f", "", "FQDN of Tanzu Hub [$HUB_TARGET]")
	rootCmd.PersistentFlags().BoolVarP(&skipTls, "skip_tls", "k", false, "Skip TLS when connecting to Tanzu Hub [$HUB_SKIP_TLS]")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Enable debug logging of HTTP requests and responses")
}