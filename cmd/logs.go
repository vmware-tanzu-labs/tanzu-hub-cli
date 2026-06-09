package cmd

import (
	"archive/tar"
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/klauspost/pgzip"
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
	logsCompress   bool
	logsFrom       string
	logsTo         string
	logsSeverities []string
	logsAppName    string
	logsDeployment string
	logsSource     string
	logsHostname   string
	logsGroup      string

	logsGetDuration string
	logsGetOutput   string
)

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Query and download logs",
}

var logsGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Query log metadata",
}

var getAppNamesCmd = &cobra.Command{
	Use:   "appnames <foundation-name>",
	Short: "List the distinct application names that produced logs",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		validateCredentials(cmd)

		foundation := args[0]

		duration, err := parseDuration(logsGetDuration)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid duration %q: %s\n", logsGetDuration, err)
			os.Exit(1)
		}

		output := strings.ToLower(logsGetOutput)
		if output != "text" && output != "json" && output != "yaml" {
			fmt.Fprintf(os.Stderr, "Invalid output %q: must be text, json, or yaml\n", logsGetOutput)
			os.Exit(1)
		}

		client, err := api.GetAccessToken(username, password, hub_url, skipTls, debug)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to authenticate: %s\n", err)
			os.Exit(1)
		}

		gql := api.NewGraphQLClient(client, hub_url, skipTls)

		// Resolve the foundation name to its entity ID (same logic as download).
		entityID, err := gql.GetFoundationEntityID(context.Background(), foundation)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to resolve foundation: %s\n", err)
			os.Exit(1)
		}

		now := time.Now().UTC()
		startTime := now.Add(-duration).Format(time.RFC3339)
		endTime := now.Format(time.RFC3339)

		names, err := gql.GetAppNames(context.Background(), entityID, startTime, endTime)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to query app names: %s\n", err)
			os.Exit(1)
		}

		if output != "text" {
			printOutput(output, outputEnvelope{Type: "appnames", Spec: struct {
				StartTime string             `json:"startTime" yaml:"startTime"`
				EndTime   string             `json:"endTime"   yaml:"endTime"`
				AppNames  []api.AppNameCount `json:"appNames"  yaml:"appNames"`
			}{StartTime: startTime, EndTime: endTime, AppNames: names}})
			return
		}

		fmt.Printf("Start: %s\n", startTime)
		fmt.Printf("End:   %s\n", endTime)
		fmt.Println()

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "APPNAME\tCOUNT")
		for _, n := range names {
			fmt.Fprintf(w, "%s\t%d\n", n.Name, n.Count)
		}
		if err := w.Flush(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to flush buffer: %s\n", err)
			os.Exit(1)
		}
	},
}

var downloadLogsCmd = &cobra.Command{
	Use:   "download <foundation-name>",
	Short: "Download log messages for a foundation to a file",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		validateCredentials(cmd)

		foundation := args[0]

		startTime, endTime, err := resolveTimeRange(cmd, logsDuration, logsFrom, logsTo)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
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

		severities, err := normalizeSeverities(logsSeverities)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			os.Exit(1)
		}

		ext := "log"
		if format == "json" {
			ext = "jsonl"
		}
		// entryBase is the base name of the log file(s); for an uncompressed
		// download it is the output file name, and inside a .tgz it names the
		// single entry (or, for very large downloads, the chunk parts).
		entryBase := fmt.Sprintf("%s-logs", foundation)

		outputFile := logsOutputFile
		if outputFile == "" {
			if logsCompress {
				outputFile = fmt.Sprintf("%s.tgz", entryBase)
			} else {
				outputFile = fmt.Sprintf("%s.%s", entryBase, ext)
			}
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

		input := api.LogInput{
			Namespace: logsNamespace,
			StartTime: startTime,
			EndTime:   endTime,
			SortOrder: order,
			QueryFilter: api.AndFilters(
				api.SeverityFilter(severities),
				api.ContainsFilter("appname", strings.TrimSpace(logsAppName)),
				api.ContainsFilter("deployment", strings.TrimSpace(logsDeployment)),
				api.ContainsFilter("source", strings.TrimSpace(logsSource)),
				api.ContainsFilter("hostname", strings.TrimSpace(logsHostname)),
				api.ContainsFilter("group", strings.TrimSpace(logsGroup)),
			),
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

		sink, err := newLogSink(outputFile, format, entryBase, ext, logsCompress)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create output: %s\n", err)
			os.Exit(1)
		}

		// Stream pages straight to the sink so records are never all held in
		// memory; the compressing sink also bounds on-disk temp usage.
		count, streamErr := gql.StreamLogs(context.Background(), entityID, input, logsMax,
			sink.WritePage,
			func(fetched int) {
				if bar != nil {
					_ = bar.Set(fetched)
				}
			},
		)
		if bar != nil {
			_ = bar.Finish()
		}

		closeErr := sink.Close()

		if streamErr != nil {
			_ = os.Remove(outputFile)
			fmt.Fprintf(os.Stderr, "Failed to query logs: %s\n", streamErr)
			os.Exit(1)
		}
		if closeErr != nil {
			_ = os.Remove(outputFile)
			fmt.Fprintf(os.Stderr, "Failed to write logs: %s\n", closeErr)
			os.Exit(1)
		}

		fmt.Printf("wrote %d log records to %s\n", count, outputFile)
	},
}

// logChunkSize is the maximum uncompressed size of a single temp chunk before
// it is flushed into the archive. Bounding the chunk size keeps both memory and
// on-disk temp usage in check for very large downloads.
const logChunkSize = 1 << 30 // 1 GiB

// logSink consumes pages of log records and writes them to an output.
type logSink interface {
	WritePage(records []api.LogRecord) error
	Close() error
}

// newLogSink returns a sink writing to outputFile. When compress is true the
// sink produces a gzip-compressed tar archive; otherwise it writes the log
// lines directly to the file.
func newLogSink(outputFile, format, entryBase, ext string, compress bool) (logSink, error) {
	if compress {
		return newTarGzSink(outputFile, format, entryBase, ext, logChunkSize)
	}
	return newPlainSink(outputFile, format)
}

// plainSink writes log lines directly to a file, streaming each page as it
// arrives.
type plainSink struct {
	f      *os.File
	w      *bufio.Writer
	format string
}

func newPlainSink(path, format string) (*plainSink, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	return &plainSink{f: f, w: bufio.NewWriter(f), format: format}, nil
}

func (s *plainSink) WritePage(records []api.LogRecord) error {
	for _, rec := range records {
		line, err := logLineBytes(rec, s.format)
		if err != nil {
			return err
		}
		if _, err := s.w.Write(line); err != nil {
			return err
		}
	}
	return nil
}

func (s *plainSink) Close() error {
	ferr := s.w.Flush()
	if cerr := s.f.Close(); ferr == nil {
		ferr = cerr
	}
	return ferr
}

// tarGzSink streams log lines into a gzip-compressed tar archive using a
// high-performance parallel gzip writer. Lines are buffered to a temp file on
// the same volume as the output; once a chunk reaches chunkSize it is flushed
// into the archive as a tar entry and the temp file is removed, so on-disk temp
// usage never exceeds roughly one chunk. A download that fits in a single chunk
// becomes one cleanly-named entry; larger downloads become ordered parts.
type tarGzSink struct {
	f         *os.File
	gz        *pgzip.Writer
	tw        *tar.Writer
	format    string
	entryBase string
	ext       string
	dir       string
	chunkSize int64

	tmp      *os.File
	tmpBytes int64
	chunks   int
}

func newTarGzSink(path, format, entryBase, ext string, chunkSize int64) (*tarGzSink, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	gz := pgzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	return &tarGzSink{
		f:         f,
		gz:        gz,
		tw:        tw,
		format:    format,
		entryBase: entryBase,
		ext:       ext,
		dir:       filepath.Dir(path),
		chunkSize: chunkSize,
	}, nil
}

func (s *tarGzSink) WritePage(records []api.LogRecord) error {
	for _, rec := range records {
		line, err := logLineBytes(rec, s.format)
		if err != nil {
			return err
		}
		if s.tmp == nil {
			tmp, terr := os.CreateTemp(s.dir, ".th-logs-*.tmp")
			if terr != nil {
				return terr
			}
			s.tmp = tmp
			s.tmpBytes = 0
		}
		if _, err := s.tmp.Write(line); err != nil {
			return err
		}
		s.tmpBytes += int64(len(line))
		if s.tmpBytes >= s.chunkSize {
			if err := s.flushChunk(false); err != nil {
				return err
			}
		}
	}
	return nil
}

// flushChunk writes the current temp file into the archive as a tar entry and
// removes it. final marks the last chunk, which (when it is also the only
// chunk) yields an unindexed entry name.
func (s *tarGzSink) flushChunk(final bool) error {
	if s.tmp == nil {
		return nil
	}
	name := s.tmp.Name()
	defer func() {
		_ = s.tmp.Close()
		_ = os.Remove(name)
		s.tmp = nil
		s.tmpBytes = 0
	}()

	if _, err := s.tmp.Seek(0, io.SeekStart); err != nil {
		return err
	}

	hdr := &tar.Header{
		Name:    s.entryName(final),
		Mode:    0o644,
		Size:    s.tmpBytes,
		ModTime: time.Now(),
	}
	if err := s.tw.WriteHeader(hdr); err != nil {
		return err
	}
	if _, err := io.Copy(s.tw, s.tmp); err != nil {
		return err
	}
	s.chunks++
	return nil
}

// entryName returns the tar entry name for the chunk about to be written. A
// lone chunk gets a clean unindexed name; multiple chunks are zero-padded so
// they sort and extract in order.
func (s *tarGzSink) entryName(final bool) string {
	if final && s.chunks == 0 {
		return fmt.Sprintf("%s.%s", s.entryBase, s.ext)
	}
	return fmt.Sprintf("%s.%06d.%s", s.entryBase, s.chunks+1, s.ext)
}

func (s *tarGzSink) Close() error {
	var ferr error
	if s.tmp != nil {
		ferr = s.flushChunk(true)
	} else if s.chunks == 0 {
		// No records at all: still emit a single empty entry.
		ferr = s.tw.WriteHeader(&tar.Header{
			Name:    fmt.Sprintf("%s.%s", s.entryBase, s.ext),
			Mode:    0o644,
			Size:    0,
			ModTime: time.Now(),
		})
	}

	if cerr := s.tw.Close(); ferr == nil {
		ferr = cerr
	}
	if cerr := s.gz.Close(); ferr == nil {
		ferr = cerr
	}
	if cerr := s.f.Close(); ferr == nil {
		ferr = cerr
	}
	return ferr
}

// parseDuration extends time.ParseDuration with a "d" (days) unit, which the
// standard library does not support. A days component, when present, must come
// first (e.g. "7d" or "7d12h"); the remainder is parsed by time.ParseDuration.
func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	i := strings.IndexByte(s, 'd')
	if i < 0 {
		return time.ParseDuration(s)
	}
	days, err := strconv.ParseFloat(s[:i], 64)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q", s)
	}
	d := time.Duration(days * 24 * float64(time.Hour))
	rest := s[i+1:]
	if rest == "" {
		return d, nil
	}
	r, err := time.ParseDuration(rest)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q: %w", s, err)
	}
	return d + r, nil
}

// logTimeLayouts are the accepted --from/--to formats, tried in order. Values
// without a timezone are interpreted as UTC.
var logTimeLayouts = []string{
	time.RFC3339,
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05",
	"2006-01-02 15:04",
	"2006-01-02",
}

// parseLogTime parses a --from/--to value into UTC, accepting the layouts in
// logTimeLayouts.
func parseLogTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	for _, layout := range logTimeLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid time %q: use RFC3339 (e.g. 2026-06-09T14:00:00Z) or 2006-01-02[ 15:04:05]", s)
}

// resolveTimeRange returns the RFC3339 start and end times for the query.
// --from/--to define an explicit range and are mutually exclusive with an
// explicitly set --duration; otherwise the range is the last duration ending
// now. When in range mode --from is required and --to defaults to now.
func resolveTimeRange(cmd *cobra.Command, durationStr, from, to string) (start, end string, err error) {
	now := time.Now().UTC()

	rangeMode := cmd.Flags().Changed("from") || cmd.Flags().Changed("to")
	if !rangeMode {
		d, perr := parseDuration(durationStr)
		if perr != nil {
			return "", "", fmt.Errorf("invalid duration %q: %w", durationStr, perr)
		}
		return now.Add(-d).Format(time.RFC3339), now.Format(time.RFC3339), nil
	}

	if cmd.Flags().Changed("duration") {
		return "", "", fmt.Errorf("--from/--to cannot be combined with --duration")
	}
	if !cmd.Flags().Changed("from") {
		return "", "", fmt.Errorf("--from is required when using --to")
	}

	startT, err := parseLogTime(from)
	if err != nil {
		return "", "", err
	}
	endT := now
	if cmd.Flags().Changed("to") {
		endT, err = parseLogTime(to)
		if err != nil {
			return "", "", err
		}
	}
	if !startT.Before(endT) {
		return "", "", fmt.Errorf("--from (%s) must be before --to (%s)", startT.Format(time.RFC3339), endT.Format(time.RFC3339))
	}
	return startT.Format(time.RFC3339), endT.Format(time.RFC3339), nil
}

// validSeverities is the set of severity values accepted by --severity.
var validSeverities = map[string]bool{
	"error":   true,
	"unknown": true,
	"info":    true,
	"debug":   true,
}

// normalizeSeverities lowercases and validates the requested severities,
// returning an error naming the first invalid value.
func normalizeSeverities(severities []string) ([]string, error) {
	out := make([]string, 0, len(severities))
	for _, s := range severities {
		s = strings.ToLower(strings.TrimSpace(s))
		if s == "" {
			continue
		}
		if !validSeverities[s] {
			return nil, fmt.Errorf("invalid severity %q: must be one of error, unknown, info, debug", s)
		}
		out = append(out, s)
	}
	return out, nil
}

// logLineBytes renders a single log record (with a trailing newline) in the
// given format ("text" or "json").
func logLineBytes(rec api.LogRecord, format string) ([]byte, error) {
	if format == "json" {
		b, err := json.Marshal(logRecordToMap(rec))
		if err != nil {
			return nil, err
		}
		return append(b, '\n'), nil
	}
	return append([]byte(formatLogLine(rec)), '\n'), nil
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
	downloadLogsCmd.Flags().StringVarP(&logsOutputFile, "file", "O", "", "Output file path (default <foundation>-logs.log, .jsonl for json, or .tgz when compressed)")
	downloadLogsCmd.Flags().StringVarP(&logsDuration, "duration", "d", "15m", "Lookback duration ending now (e.g. 15m, 1h, 24h, 7d); mutually exclusive with --from/--to")
	downloadLogsCmd.Flags().StringVar(&logsFrom, "from", "", "Start of an explicit time range (RFC3339 or 2006-01-02[ 15:04:05]); alternative to --duration")
	downloadLogsCmd.Flags().StringVar(&logsTo, "to", "", "End of an explicit time range (defaults to now); requires --from")
	downloadLogsCmd.Flags().StringVarP(&logsNamespace, "namespace", "n", "logs", "Log namespace to query")
	downloadLogsCmd.Flags().IntVar(&logsMax, "max", 0, "Maximum number of log records to fetch across all pages (0 = all)")
	downloadLogsCmd.Flags().StringVar(&logsOrder, "order", "DESC", "Sort order: ASC or DESC")
	downloadLogsCmd.Flags().StringVar(&logsFormat, "format", "text", "Output format: text or json")
	downloadLogsCmd.Flags().BoolVar(&logsCompress, "compress", false, "Stream-compress output into a .tgz archive")
	downloadLogsCmd.Flags().StringSliceVar(&logsSeverities, "severity", nil, "Only include these severities (repeatable or comma-separated): error, unknown, info, debug")
	downloadLogsCmd.Flags().StringVar(&logsAppName, "appname", "", "Only include records whose appname contains this substring")
	downloadLogsCmd.Flags().StringVar(&logsDeployment, "deployment", "", "Only include records whose deployment contains this substring")
	downloadLogsCmd.Flags().StringVar(&logsSource, "source", "", "Only include records whose source contains this substring")
	downloadLogsCmd.Flags().StringVar(&logsHostname, "hostname", "", "Only include records whose hostname contains this substring")
	downloadLogsCmd.Flags().StringVar(&logsGroup, "group", "", "Only include records whose group contains this substring")

	logsCmd.AddCommand(logsGetCmd)
	logsGetCmd.AddCommand(getAppNamesCmd)
	getAppNamesCmd.Flags().StringVarP(&logsGetDuration, "duration", "d", "7d", "Lookback duration (e.g. 1h, 24h, 7d)")
	getAppNamesCmd.Flags().StringVarP(&logsGetOutput, "output", "o", "text", "Output format: text, json, yaml")
}
