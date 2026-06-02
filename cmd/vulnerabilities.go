package cmd

import (
	"fmt"
	"os"
	"slices"

	"github.com/laidbackware/tanzu-hub-cli/api"
	"github.com/spf13/cobra"
)

var (
	filePath string
	fileType string
)

var vulnerabilitiesCmd = &cobra.Command{
	Use:   "vulnerabilities",
	Short: "Upload vulnerability or SBOM data to Tanzu Hub",
}

var uploadCmd = &cobra.Command{
	Use:   "upload",
	Short: "Upload vulnerability or SBOM data to Tanzu Hub",
	Run: func(cmd *cobra.Command, args []string) {
		// Validate fileType
		validOptions := []string{"vulnerability", "sbom"}
		if !slices.Contains(validOptions, fileType) {
			fmt.Fprintln(os.Stderr, "File type must be (vulnerability, sbom)")
			os.Exit(1)
		}
		
		validateCredentials(cmd)

		client, err := api.GetAccessToken(username, password, hub_url, skipTls, debug)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to authenticate: %s\n", err)
			os.Exit(1)
		}

		err = client.UploadVulnerabilities(filePath, fileType)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Upload failed: %s\n", err)
			os.Exit(1)
		}

		fmt.Println("\nUpload successful")
	},
}

func init() {
	rootCmd.AddCommand(vulnerabilitiesCmd)
	vulnerabilitiesCmd.AddCommand(uploadCmd)
	uploadCmd.Flags().StringVarP(&filePath, "file", "", "", "Path to the zip file to upload")
	uploadCmd.Flags().StringVarP(&fileType, "type", "", "vulnerability", "Type of upload: 'vulnerability' or 'sbom'")
	_ = uploadCmd.MarkFlagRequired("file")
	_ = uploadCmd.MarkFlagRequired("type")
}
