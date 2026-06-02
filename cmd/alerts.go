package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/laidbackware/tanzu-hub-cli/api"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var alertOutputFormat string
var alertStripID bool
var alertSkipSystem bool

var alertCmd = &cobra.Command{
	Use:   "alert",
	Short: "Manage observability alerts",
}

var listAlertsCmd = &cobra.Command{
	Use:   "list",
	Short: "List metric alerts from Tanzu Hub",
	Run: func(cmd *cobra.Command, args []string) {
		validateCredentials(cmd)

		client, err := api.GetAccessToken(username, password, hub_url, skipTls, debug)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to authenticate: %s\n", err)
			os.Exit(1)
		}

		gql := api.NewGraphQLClient(client, hub_url, skipTls)
		result, err := gql.QueryObservabilityAlerts(
			context.Background(),
			api.ObservabilityAlertFilterInput{AlertType: "METRIC"},
			[]api.QuerySort{{Field: "creationTime", Order: "DESC"}},
			100, "", "",
		)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to query alerts: %s\n", err)
			os.Exit(1)
		}

		alerts := result.Alerts
		if alertSkipSystem {
			alerts = filterCustomAlerts(alerts)
		}

		if alertOutputFormat != "text" {
			if alertStripID {
				alerts = stripAlertIDs(alerts)
			}
			printOutput(alertOutputFormat, outputEnvelope{Type: "alerts", Spec: alerts})
			return
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tID\tSTATUS\tENABLED\tSEVERITY\tLAST RUN")
		for _, a := range alerts {
			fmt.Fprintf(w, "%s\t%s\t%s\t%t\t%s\t%s\n",
				truncate(a.PolicyName, 54),
				a.PolicyID,
				a.AlertStatus,
				a.Enabled,
				a.Rule.AlertCondition.Threshold.AlertSeverity,
				a.LastRunTime,
			)
		}
		if err = w.Flush(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to flush buffer: %s\n", err)
			os.Exit(1)
		}
	},
}

var applyAlertFile string

var applyAlertCmd = &cobra.Command{
	Use:   "apply",
	Short: "Create or update metric alerts from a YAML or JSON file",
	Run: func(cmd *cobra.Command, args []string) {
		validateCredentials(cmd)

		var data []byte
		var err error
		if applyAlertFile != "" {
			data, err = os.ReadFile(applyAlertFile)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to read file: %s\n", err)
				os.Exit(1)
			}
		} else {
			data, err = io.ReadAll(os.Stdin)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to read stdin: %s\n", err)
				os.Exit(1)
			}
		}

		var envelope struct {
			Spec []api.ObservabilityMetricAlert `yaml:"spec"`
		}
		if err = yaml.Unmarshal(data, &envelope); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to parse input: %s\n", err)
			os.Exit(1)
		}

		if len(envelope.Spec) == 0 {
			fmt.Fprintln(os.Stderr, "No alerts found in input")
			os.Exit(1)
		}

		client, err := api.GetAccessToken(username, password, hub_url, skipTls, debug)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to authenticate: %s\n", err)
			os.Exit(1)
		}

		gql := api.NewGraphQLClient(client, hub_url, skipTls)

		// Build name→existing alert mapping for ID resolution.
		existing, err := gql.QueryObservabilityAlerts(
			context.Background(),
			api.ObservabilityAlertFilterInput{AlertType: "METRIC"},
			[]api.QuerySort{{Field: "creationTime", Order: "DESC"}},
			100, "", "",
		)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to query existing alerts: %s\n", err)
			os.Exit(1)
		}
		alertsByName, duplicates := buildAlertNameIndex(existing.Alerts)
		if err = validateAlertNames(envelope.Spec, duplicates); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			os.Exit(1)
		}

		hasErrors := false
		for _, a := range envelope.Spec {
			if !a.IsCustomAlert {
				fmt.Printf("skipped %s (%s): not a custom alert\n", a.PolicyName, a.PolicyID)
				continue
			}
			if a.PolicyID == "" {
				if match, ok := alertsByName[a.PolicyName]; ok {
					a.PolicyID = match.PolicyID
					a.NodeVersion = match.NodeVersion
					fmt.Printf("resolved %q to existing ID %s\n", a.PolicyName, a.PolicyID)
				}
			}
			if a.PolicyID != "" {
				result, opErr := gql.UpdateMetricAlert(context.Background(), api.AlertToUpdateInput(a))
				if opErr != nil {
					fmt.Fprintf(os.Stderr, "Failed to update alert %q (%s): %s\n", a.PolicyName, a.PolicyID, opErr)
					hasErrors = true
					continue
				}
				fmt.Printf("updated %s (%s)\n", result.PolicyName, result.PolicyID)
			} else {
				result, opErr := gql.CreateMetricAlert(context.Background(), api.AlertToCreateInput(a))
				if opErr != nil {
					fmt.Fprintf(os.Stderr, "Failed to create alert %q: %s\n", a.PolicyName, opErr)
					hasErrors = true
					continue
				}
				fmt.Printf("created %s (%s)\n", result.PolicyName, result.PolicyID)
			}
		}

		if hasErrors {
			os.Exit(1)
		}
	},
}

var getAlertCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get full details of a single alert by ID",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		validateCredentials(cmd)

		client, err := api.GetAccessToken(username, password, hub_url, skipTls, debug)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to authenticate: %s\n", err)
			os.Exit(1)
		}

		gql := api.NewGraphQLClient(client, hub_url, skipTls)
		result, err := gql.QueryObservabilityAlerts(
			context.Background(),
			api.ObservabilityAlertFilterInput{PolicyID: []string{args[0]}},
			[]api.QuerySort{{Field: "creationTime", Order: "DESC"}},
			1, "", "",
		)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to query alert: %s\n", err)
			os.Exit(1)
		}

		if len(result.Alerts) == 0 {
			fmt.Fprintf(os.Stderr, "No alert found with ID: %s\n", args[0])
			os.Exit(1)
		}

		a := result.Alerts[0]

		if alertOutputFormat != "text" {
			printOutput(alertOutputFormat, outputEnvelope{Type: "alert", Spec: []api.ObservabilityMetricAlert{a}})
			return
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "Name:\t%s\n", a.PolicyName)
		fmt.Fprintf(w, "ID:\t%s\n", a.PolicyID)
		fmt.Fprintf(w, "Description:\t%s\n", stripHTML(a.Description))
		fmt.Fprintf(w, "Status:\t%s\n", a.AlertStatus)
		fmt.Fprintf(w, "Enabled:\t%t\n", a.Enabled)
		fmt.Fprintf(w, "Type:\t%s\n", a.AlertType)
		fmt.Fprintf(w, "Entity Type:\t%s\n", a.EntityType)
		fmt.Fprintf(w, "Source:\t%s\n", a.Source)
		fmt.Fprintf(w, "Policy Provider:\t%s\n", a.PolicyProvider)
		fmt.Fprintf(w, "Is Custom:\t%t\n", a.IsCustomAlert)
		fmt.Fprintf(w, "Severity:\t%s\n", a.Rule.AlertCondition.Threshold.AlertSeverity)
		fmt.Fprintf(w, "Threshold Value:\t%g\n", a.Rule.AlertCondition.Threshold.ThresholdValue)
		fmt.Fprintf(w, "Operator:\t%s\n", a.Rule.AlertCondition.Operator)
		fmt.Fprintf(w, "Query:\t%s\n", a.Rule.QueryString)
		fmt.Fprintf(w, "Namespace:\t%s\n", a.Rule.Namespace)
		fmt.Fprintf(w, "Dashboard ID:\t%s\n", a.DashboardID)
		fmt.Fprintf(w, "Runbook Template ID:\t%s\n", a.RunbookTemplateID)
		fmt.Fprintf(w, "Entity ID:\t%s\n", a.EntityInfo.EntityID)
		fmt.Fprintf(w, "Entity Name:\t%s\n", a.EntityInfo.EntityName)
		fmt.Fprintf(w, "Created By:\t%s\n", a.CreatedBy.UserAccount)
		fmt.Fprintf(w, "Creation Time:\t%s\n", a.CreationTime)
		fmt.Fprintf(w, "Updated By:\t%s\n", a.UpdatedBy.UserAccount)
		fmt.Fprintf(w, "Last Updated:\t%s\n", a.LastUpdateTime)
		fmt.Fprintf(w, "Last Run:\t%s\n", a.LastRunTime)
		fmt.Fprintf(w, "Node Version:\t%s\n", a.NodeVersion)
		if err = w.Flush(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to flush buffer: %s\n", err)
			os.Exit(1)
		}
	},
}

var deleteAlertCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete an alert by ID",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		validateCredentials(cmd)

		client, err := api.GetAccessToken(username, password, hub_url, skipTls, debug)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to authenticate: %s\n", err)
			os.Exit(1)
		}

		gql := api.NewGraphQLClient(client, hub_url, skipTls)

		// Fetch the alert to obtain its nodeVersion.
		result, err := gql.QueryObservabilityAlerts(
			context.Background(),
			api.ObservabilityAlertFilterInput{PolicyID: []string{args[0]}},
			[]api.QuerySort{{Field: "creationTime", Order: "DESC"}},
			1, "", "",
		)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to query alert: %s\n", err)
			os.Exit(1)
		}
		if len(result.Alerts) == 0 {
			fmt.Fprintf(os.Stderr, "No alert found with ID: %s\n", args[0])
			os.Exit(1)
		}

		a := result.Alerts[0]
		deleteResult, err := gql.DeleteAlerts(context.Background(), []api.HubPolicyIdInput{
			{PolicyID: a.PolicyID, NodeVersion: a.NodeVersion},
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to delete alert: %s\n", err)
			os.Exit(1)
		}

		if deleteResult.FailureCount > 0 {
			fmt.Fprintf(os.Stderr, "Failed to delete alert %s (%s): server reported %d failures\n", a.PolicyName, a.PolicyID, deleteResult.FailureCount)
			os.Exit(1)
		}
		fmt.Printf("deleted %s (%s)\n", a.PolicyName, a.PolicyID)
	},
}

func runAlertStatusCmd(cmd *cobra.Command, alertID string, enabled bool) {
	validateCredentials(cmd)

	client, err := api.GetAccessToken(username, password, hub_url, skipTls, debug)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to authenticate: %s\n", err)
		os.Exit(1)
	}

	gql := api.NewGraphQLClient(client, hub_url, skipTls)

	result, err := gql.QueryObservabilityAlerts(
		context.Background(),
		api.ObservabilityAlertFilterInput{PolicyID: []string{alertID}},
		[]api.QuerySort{{Field: "creationTime", Order: "DESC"}},
		1, "", "",
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to query alert: %s\n", err)
		os.Exit(1)
	}
	if len(result.Alerts) == 0 {
		fmt.Fprintf(os.Stderr, "No alert found with ID: %s\n", alertID)
		os.Exit(1)
	}

	a := result.Alerts[0]
	statusResult, err := gql.UpdateAlertStatus(context.Background(), api.ObservabilityAlertStatusUpdateInput{
		AlertType: a.AlertType,
		Input: []api.HubPolicyIdInput{
			{PolicyID: a.PolicyID, NodeVersion: a.NodeVersion},
		},
		Status: enabled,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to update alert status: %s\n", err)
		os.Exit(1)
	}
	if statusResult.FailureCount > 0 {
		fmt.Fprintf(os.Stderr, "Failed to update alert %s (%s): server reported %d failures\n", a.PolicyName, a.PolicyID, statusResult.FailureCount)
		os.Exit(1)
	}

	action := "disabled"
	if enabled {
		action = "enabled"
	}
	fmt.Printf("%s %s (%s)\n", action, a.PolicyName, a.PolicyID)
}

var enableAlertCmd = &cobra.Command{
	Use:   "enable <id>",
	Short: "Enable an alert by ID",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		runAlertStatusCmd(cmd, args[0], true)
	},
}

var disableAlertCmd = &cobra.Command{
	Use:   "disable <id>",
	Short: "Disable an alert by ID",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		runAlertStatusCmd(cmd, args[0], false)
	},
}

// stripAlertIDs returns a copy of the alerts with all ID fields cleared.
func stripAlertIDs(alerts []api.ObservabilityMetricAlert) []api.ObservabilityMetricAlert {
	out := make([]api.ObservabilityMetricAlert, len(alerts))
	for i, a := range alerts {
		a.PolicyID = ""
		a.NodeVersion = ""
		a.RunbookTemplateID = ""
		a.DashboardID = ""
		a.EntityInfo.EntityID = ""
		out[i] = a
	}
	return out
}

// filterCustomAlerts returns only alerts where IsCustomAlert is true.
func filterCustomAlerts(alerts []api.ObservabilityMetricAlert) []api.ObservabilityMetricAlert {
	out := make([]api.ObservabilityMetricAlert, 0, len(alerts))
	for _, a := range alerts {
		if a.IsCustomAlert {
			out = append(out, a)
		}
	}
	return out
}

// buildAlertNameIndex builds a name→alert mapping and tracks names that appear
// more than once on the server.
func buildAlertNameIndex(alerts []api.ObservabilityMetricAlert) (byName map[string]api.ObservabilityMetricAlert, duplicates map[string]bool) {
	byName = make(map[string]api.ObservabilityMetricAlert, len(alerts))
	duplicates = make(map[string]bool)
	for _, a := range alerts {
		if _, seen := byName[a.PolicyName]; seen {
			duplicates[a.PolicyName] = true
		}
		byName[a.PolicyName] = a
	}
	return byName, duplicates
}

// validateAlertNames checks that alerts relying on name-based resolution
// (no policyId) can be unambiguously matched. It returns an error if any
// name appears more than once on the server or within the input itself.
func validateAlertNames(specs []api.ObservabilityMetricAlert, duplicates map[string]bool) error {
	inputNames := make(map[string]bool, len(specs))
	for _, a := range specs {
		if a.PolicyID != "" {
			continue
		}
		if duplicates[a.PolicyName] {
			return fmt.Errorf("multiple existing alerts share the name %q; either add the ID or rename the duplicate", a.PolicyName)
		}
		if inputNames[a.PolicyName] {
			return fmt.Errorf("input contains duplicate alert name %q without policyId; names must be unique for declarative applies", a.PolicyName)
		}
		inputNames[a.PolicyName] = true
	}
	return nil
}

func init() {
	rootCmd.AddCommand(alertCmd)
	alertCmd.AddCommand(listAlertsCmd)
	listAlertsCmd.Flags().StringVarP(&alertOutputFormat, "output", "o", "text", "Output format: text, json, yaml")
	listAlertsCmd.Flags().BoolVar(&alertStripID, "strip-id", false, "Remove all ID fields from output")
	listAlertsCmd.Flags().BoolVar(&alertSkipSystem, "skip-system", false, "Exclude system-managed alerts (isCustomAlert=false)")
	alertCmd.AddCommand(getAlertCmd)
	getAlertCmd.Flags().StringVarP(&alertOutputFormat, "output", "o", "text", "Output format: text, json, yaml")
	alertCmd.AddCommand(deleteAlertCmd)
	alertCmd.AddCommand(enableAlertCmd)
	alertCmd.AddCommand(disableAlertCmd)
	alertCmd.AddCommand(applyAlertCmd)
	applyAlertCmd.Flags().StringVarP(&applyAlertFile, "file", "", "", "Path to YAML or JSON file (reads stdin if omitted)")
}
