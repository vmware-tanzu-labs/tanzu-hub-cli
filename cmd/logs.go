package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/laidbackware/tanzu-hub-cli/api"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	logsOutputFile string
	logsDuration   string
	logsNamespace  string
	logsMax        int
	logsOrder      string
	logsFormat     string
)

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Query and download logs",
}

var downloadLogsCmd = &cobra.Command{
	Use:   "download <foundation-name>",
	Short: "Download log messages for a foundation to a file",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		validateCredentials(cmd)

		foundation := args[0]

		duration, err := time.ParseDuration(logsDuration)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid duration %q: %s\n", logsDuration, err)
			os.Exit(1)
		}

		order := strings.ToUpper(logsOrder)
		if order != "ASC" && order != "DESC" {
			fmt.Fprintf(os.Stderr, "Invalid order %q: must be ASC or DESC\n", logsOrder)
			os.Exit(1)
		}

		format := strings.ToLower(logsFormat)
		if format != "text" && format != "json" {
			fmt.Fprintf(os.Stderr, "Invalid format %q: must be text or json\n", logsFormat)
			os.Exit(1)
		}

		outputFile := logsOutputFile
		if outputFile == "" {
			ext := "log"
			if format == "json" {
				ext = "jsonl"
			}
			outputFile = fmt.Sprintf("%s-logs.%s", foundation, ext)
		}

		client, err := api.GetAccessToken(username, password, hub_url, skipTls, debug)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to authenticate: %s\n", err)
			os.Exit(1)
		}

		gql := api.NewGraphQLClient(client, hub_url, skipTls)

		// Resolve the foundation name to its entity ID.
		entityID, err := gql.GetFoundationEntityID(context.Background(), foundation)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to resolve foundation: %s\n", err)
			os.Exit(1)
		}

		now := time.Now().UTC()
		input := api.LogInput{
			Namespace: logsNamespace,
			StartTime: now.Add(-duration).Format(time.RFC3339),
			EndTime:   now.Format(time.RFC3339),
			SortOrder: order,
		}

		var bar *progressbar.ProgressBar
		if term.IsTerminal(int(os.Stderr.Fd())) {
			// Determine the total up front so the bar is determinate. The count
			// aggregation is an extra round trip, so only do it when a bar will
			// actually be shown.
			total := -1
			if c, cerr := gql.GetLogCount(context.Background(), entityID, input); cerr == nil && c > 0 {
				total = c
			}
			if logsMax > 0 && (total < 0 || logsMax < total) {
				total = logsMax
			}
			bar = newLogsProgressBar(total)
		}

		result, err := gql.QueryLogs(context.Background(), entityID, input, logsMax, func(fetched int) {
			if bar != nil {
				_ = bar.Set(fetched)
			}
		})
		if bar != nil {
			_ = bar.Finish()
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to query logs: %s\n", err)
			os.Exit(1)
		}

		count, err := writeLogsToFile(outputFile, result.LogRecords, format)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write logs: %s\n", err)
			os.Exit(1)
		}

		fmt.Printf("wrote %d log records to %s\n", count, outputFile)
	},
}

// writeLogsToFile writes log records to path in the given format ("text" or
// "json"). The text format emits one line per record as
// "deployment - appname - hostname - severity - text"; the json format emits
// one JSON object per line (JSONL) mapping field keys to values. It returns the
// number of records written.
func writeLogsToFile(path string, records []api.LogRecord, format string) (int, error) {
	f, err := os.Create(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	for _, rec := range records {
		var line []byte
		if format == "json" {
			line, err = json.Marshal(logRecordToMap(rec))
			if err != nil {
				return 0, err
			}
		} else {
			line = []byte(formatLogLine(rec))
		}
		if _, err := w.Write(line); err != nil {
			return 0, err
		}
		if err := w.WriteByte('\n'); err != nil {
			return 0, err
		}
	}
	if err := w.Flush(); err != nil {
		return 0, err
	}

	return len(records), nil
}

// newLogsProgressBar returns a progress bar that renders to stderr. When total
// is positive the bar is determinate; otherwise (total < 0) it shows an
// indeterminate spinner with a running record count.
func newLogsProgressBar(total int) *progressbar.ProgressBar {
	return progressbar.NewOptions(total,
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionSetDescription("Downloading logs"),
		progressbar.OptionShowCount(),
		progressbar.OptionSpinnerType(14),
		progressbar.OptionClearOnFinish(),
	)
}

// formatLogLine renders a log record as
// "deployment - appname - hostname - severity - text".
func formatLogLine(rec api.LogRecord) string {
	m := logRecordToMap(rec)
	return strings.Join([]string{
		m["deployment"],
		m["appname"],
		m["hostname"],
		m["severity"],
		m["text"],
	}, " - ")
}

// logRecordToMap flattens a log record's fields into a key→value map.
func logRecordToMap(rec api.LogRecord) map[string]string {
	m := make(map[string]string, len(rec.Fields))
	for _, fld := range rec.Fields {
		m[fld.Key] = fld.Value
	}
	return m
}

func init() {
	rootCmd.AddCommand(logsCmd)
	logsCmd.AddCommand(downloadLogsCmd)
	downloadLogsCmd.Flags().StringVarP(&logsOutputFile, "file", "O", "", "Output file path (default <foundation>-logs.log, or .jsonl for json format)")
	downloadLogsCmd.Flags().StringVarP(&logsDuration, "duration", "d", "15m", "Lookback duration (e.g. 15m, 1h, 24h)")
	downloadLogsCmd.Flags().StringVarP(&logsNamespace, "namespace", "n", "logs", "Log namespace to query")
	downloadLogsCmd.Flags().IntVar(&logsMax, "max", 0, "Maximum number of log records to fetch across all pages (0 = all)")
	downloadLogsCmd.Flags().StringVar(&logsOrder, "order", "DESC", "Sort order: ASC or DESC")
	downloadLogsCmd.Flags().StringVar(&logsFormat, "format", "text", "Output format: text or json")
}
