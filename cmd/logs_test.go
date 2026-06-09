package cmd

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/laidbackware/tanzu-hub-cli/api"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseDuration(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   string
		want time.Duration
	}{
		{"15m", 15 * time.Minute},
		{"24h", 24 * time.Hour},
		{"7d", 7 * 24 * time.Hour},
		{"1d12h", 36 * time.Hour},
		{"0d", 0},
		{" 2d ", 2 * 24 * time.Hour},
	}
	for _, c := range cases {
		got, err := parseDuration(c.in)
		require.NoError(t, err, c.in)
		assert.Equal(t, c.want, got, c.in)
	}

	// Invalid inputs are rejected.
	for _, bad := range []string{"d", "xd", "7days", "7d3x"} {
		_, err := parseDuration(bad)
		assert.Error(t, err, bad)
	}
}

func TestParseLogTime(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   string
		want string // expected RFC3339 in UTC
	}{
		{"2026-06-09T14:00:00Z", "2026-06-09T14:00:00Z"},
		{"2026-06-09T14:00:00", "2026-06-09T14:00:00Z"},
		{"2026-06-09 14:00:00", "2026-06-09T14:00:00Z"},
		{"2026-06-09 14:00", "2026-06-09T14:00:00Z"},
		{"2026-06-09", "2026-06-09T00:00:00Z"},
		{" 2026-06-09 ", "2026-06-09T00:00:00Z"},
	}
	for _, c := range cases {
		got, err := parseLogTime(c.in)
		require.NoError(t, err, c.in)
		assert.Equal(t, c.want, got.Format(time.RFC3339), c.in)
	}

	for _, bad := range []string{"", "nonsense", "06/09/2026", "2026-13-01"} {
		_, err := parseLogTime(bad)
		assert.Error(t, err, bad)
	}
}

// newDownloadCmd builds a fresh command carrying only the flags resolveTimeRange
// inspects, so each test can set them independently.
func newDownloadCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "download"}
	cmd.Flags().String("duration", "15m", "")
	cmd.Flags().String("from", "", "")
	cmd.Flags().String("to", "", "")
	return cmd
}

func TestResolveTimeRange_Duration(t *testing.T) {
	t.Parallel()

	cmd := newDownloadCmd()
	require.NoError(t, cmd.Flags().Set("duration", "1h"))

	start, end, err := resolveTimeRange(cmd, "1h", "", "")
	require.NoError(t, err)

	startT, err := time.Parse(time.RFC3339, start)
	require.NoError(t, err)
	endT, err := time.Parse(time.RFC3339, end)
	require.NoError(t, err)
	assert.InDelta(t, time.Hour.Seconds(), endT.Sub(startT).Seconds(), 2)
}

func TestResolveTimeRange_FromTo(t *testing.T) {
	t.Parallel()

	cmd := newDownloadCmd()
	require.NoError(t, cmd.Flags().Set("from", "2026-06-01T00:00:00Z"))
	require.NoError(t, cmd.Flags().Set("to", "2026-06-02T00:00:00Z"))

	start, end, err := resolveTimeRange(cmd, "15m", "2026-06-01T00:00:00Z", "2026-06-02T00:00:00Z")
	require.NoError(t, err)
	assert.Equal(t, "2026-06-01T00:00:00Z", start)
	assert.Equal(t, "2026-06-02T00:00:00Z", end)
}

func TestResolveTimeRange_FromDefaultsToNow(t *testing.T) {
	t.Parallel()

	cmd := newDownloadCmd()
	require.NoError(t, cmd.Flags().Set("from", "2020-01-01T00:00:00Z"))

	start, end, err := resolveTimeRange(cmd, "15m", "2020-01-01T00:00:00Z", "")
	require.NoError(t, err)
	assert.Equal(t, "2020-01-01T00:00:00Z", start)
	endT, err := time.Parse(time.RFC3339, end)
	require.NoError(t, err)
	assert.WithinDuration(t, time.Now().UTC(), endT, 5*time.Second)
}

func TestResolveTimeRange_DurationAndRangeConflict(t *testing.T) {
	t.Parallel()

	cmd := newDownloadCmd()
	require.NoError(t, cmd.Flags().Set("duration", "1h"))
	require.NoError(t, cmd.Flags().Set("from", "2026-06-01T00:00:00Z"))

	_, _, err := resolveTimeRange(cmd, "1h", "2026-06-01T00:00:00Z", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be combined")
}

func TestResolveTimeRange_ToRequiresFrom(t *testing.T) {
	t.Parallel()

	cmd := newDownloadCmd()
	require.NoError(t, cmd.Flags().Set("to", "2026-06-02T00:00:00Z"))

	_, _, err := resolveTimeRange(cmd, "15m", "", "2026-06-02T00:00:00Z")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--from is required")
}

func TestResolveTimeRange_FromAfterTo(t *testing.T) {
	t.Parallel()

	cmd := newDownloadCmd()
	require.NoError(t, cmd.Flags().Set("from", "2026-06-02T00:00:00Z"))
	require.NoError(t, cmd.Flags().Set("to", "2026-06-01T00:00:00Z"))

	_, _, err := resolveTimeRange(cmd, "15m", "2026-06-02T00:00:00Z", "2026-06-01T00:00:00Z")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be before")
}

func TestLogRecordToMap(t *testing.T) {
	t.Parallel()

	rec := api.LogRecord{
		Fields: []api.LogField{
			{Key: "severity", Value: "info"},
			{Key: "text", Value: "hello world"},
			{Key: "foundation", Value: "ops.eval.lab"},
		},
	}

	m := logRecordToMap(rec)
	assert.Len(t, m, 3)
	assert.Equal(t, "info", m["severity"])
	assert.Equal(t, "hello world", m["text"])
	assert.Equal(t, "ops.eval.lab", m["foundation"])
}

func TestLogRecordToMap_Empty(t *testing.T) {
	t.Parallel()

	m := logRecordToMap(api.LogRecord{})
	assert.Empty(t, m)
}

func TestFormatLogLine(t *testing.T) {
	t.Parallel()

	rec := api.LogRecord{
		Fields: []api.LogField{
			{Key: "deployment", Value: "cf-abc"},
			{Key: "appname", Value: "gorouter"},
			{Key: "hostname", Value: "10.0.0.1"},
			{Key: "severity", Value: "info"},
			{Key: "text", Value: "request handled"},
			{Key: "extra", Value: "ignored"},
		},
	}

	assert.Equal(t, "cf-abc - gorouter - 10.0.0.1 - info - request handled", formatLogLine(rec))
}

func TestFormatLogLine_MissingFields(t *testing.T) {
	t.Parallel()

	rec := api.LogRecord{
		Fields: []api.LogField{
			{Key: "severity", Value: "error"},
			{Key: "text", Value: "boom"},
		},
	}

	// Missing fields render as empty segments, preserving column positions.
	assert.Equal(t, " -  -  - error - boom", formatLogLine(rec))
}

// writePagesToSink writes each page to the sink and closes it.
func writePagesToSink(t *testing.T, s logSink, pages ...[]api.LogRecord) {
	t.Helper()
	for _, p := range pages {
		require.NoError(t, s.WritePage(p))
	}
	require.NoError(t, s.Close())
}

func TestPlainSink_Text(t *testing.T) {
	t.Parallel()

	records := []api.LogRecord{
		{Fields: []api.LogField{
			{Key: "deployment", Value: "cf-1"},
			{Key: "appname", Value: "gorouter"},
			{Key: "hostname", Value: "10.0.0.1"},
			{Key: "severity", Value: "info"},
			{Key: "text", Value: "line one"},
		}},
		{Fields: []api.LogField{
			{Key: "deployment", Value: "cf-2"},
			{Key: "appname", Value: "uaa"},
			{Key: "hostname", Value: "10.0.0.2"},
			{Key: "severity", Value: "error"},
			{Key: "text", Value: "line two"},
		}},
	}

	path := filepath.Join(t.TempDir(), "out.log")
	s, err := newPlainSink(path, "text")
	require.NoError(t, err)
	// Two separate pages exercise streaming across calls.
	writePagesToSink(t, s, records[:1], records[1:])

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	require.Len(t, lines, 2)
	assert.Equal(t, "cf-1 - gorouter - 10.0.0.1 - info - line one", lines[0])
	assert.Equal(t, "cf-2 - uaa - 10.0.0.2 - error - line two", lines[1])
}

func TestPlainSink_JSON(t *testing.T) {
	t.Parallel()

	records := []api.LogRecord{
		{Fields: []api.LogField{{Key: "text", Value: "line one"}, {Key: "severity", Value: "info"}}},
		{Fields: []api.LogField{{Key: "text", Value: "line two"}, {Key: "severity", Value: "error"}}},
	}

	path := filepath.Join(t.TempDir(), "out.jsonl")
	s, err := newPlainSink(path, "json")
	require.NoError(t, err)
	writePagesToSink(t, s, records)

	f, err := os.Open(path)
	require.NoError(t, err)
	defer func() { _ = f.Close() }()

	var lines []map[string]string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var m map[string]string
		require.NoError(t, json.Unmarshal(scanner.Bytes(), &m))
		lines = append(lines, m)
	}
	require.NoError(t, scanner.Err())

	require.Len(t, lines, 2)
	assert.Equal(t, "line one", lines[0]["text"])
	assert.Equal(t, "info", lines[0]["severity"])
	assert.Equal(t, "line two", lines[1]["text"])
	assert.Equal(t, "error", lines[1]["severity"])
}

// readTarGzEntry reads the named entry from a gzip-compressed tar at path.
func readTarGzEntry(t *testing.T, path, entryName string) string {
	t.Helper()
	f, err := os.Open(path)
	require.NoError(t, err)
	defer func() { _ = f.Close() }()

	gz, err := gzip.NewReader(f)
	require.NoError(t, err)
	defer func() { _ = gz.Close() }()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		if hdr.Name == entryName {
			data, err := io.ReadAll(tr)
			require.NoError(t, err)
			// The declared header size must match the actual content.
			assert.Equal(t, hdr.Size, int64(len(data)))
			return string(data)
		}
	}
	t.Fatalf("entry %q not found in %s", entryName, path)
	return ""
}

// readAllTarGz returns the entry names (in archive order) and their
// concatenated content from a gzip-compressed tar at path.
func readAllTarGz(t *testing.T, path string) (names []string, content string) {
	t.Helper()
	f, err := os.Open(path)
	require.NoError(t, err)
	defer func() { _ = f.Close() }()

	gz, err := gzip.NewReader(f)
	require.NoError(t, err)
	defer func() { _ = gz.Close() }()

	var sb strings.Builder
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		data, err := io.ReadAll(tr)
		require.NoError(t, err)
		assert.Equal(t, hdr.Size, int64(len(data)))
		names = append(names, hdr.Name)
		sb.Write(data)
	}
	return names, sb.String()
}

func newTarGzTextRecords() []api.LogRecord {
	return []api.LogRecord{
		{Fields: []api.LogField{
			{Key: "deployment", Value: "cf-1"},
			{Key: "appname", Value: "gorouter"},
			{Key: "hostname", Value: "10.0.0.1"},
			{Key: "severity", Value: "info"},
			{Key: "text", Value: "line one"},
		}},
		{Fields: []api.LogField{
			{Key: "deployment", Value: "cf-2"},
			{Key: "appname", Value: "uaa"},
			{Key: "hostname", Value: "10.0.0.2"},
			{Key: "severity", Value: "error"},
			{Key: "text", Value: "line two"},
		}},
	}
}

func TestTarGzSink_SingleEntry(t *testing.T) {
	t.Parallel()

	records := newTarGzTextRecords()

	path := filepath.Join(t.TempDir(), "out.tgz")
	// A large chunk size keeps everything in one chunk → one clean entry.
	s, err := newTarGzSink(path, "text", "myfoundation-logs", "log", logChunkSize)
	require.NoError(t, err)
	writePagesToSink(t, s, records)

	content := readTarGzEntry(t, path, "myfoundation-logs.log")
	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	require.Len(t, lines, 2)
	assert.Equal(t, "cf-1 - gorouter - 10.0.0.1 - info - line one", lines[0])
	assert.Equal(t, "cf-2 - uaa - 10.0.0.2 - error - line two", lines[1])
}

func TestTarGzSink_JSON(t *testing.T) {
	t.Parallel()

	records := []api.LogRecord{
		{Fields: []api.LogField{{Key: "text", Value: "line one"}, {Key: "severity", Value: "info"}}},
	}

	path := filepath.Join(t.TempDir(), "out.tgz")
	s, err := newTarGzSink(path, "json", "f-logs", "jsonl", logChunkSize)
	require.NoError(t, err)
	writePagesToSink(t, s, records)

	content := readTarGzEntry(t, path, "f-logs.jsonl")
	var m map[string]string
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(content)), &m))
	assert.Equal(t, "line one", m["text"])
	assert.Equal(t, "info", m["severity"])
}

func TestTarGzSink_Chunked(t *testing.T) {
	t.Parallel()

	records := newTarGzTextRecords()

	path := filepath.Join(t.TempDir(), "out.tgz")
	// A 1-byte chunk size forces every record into its own chunk/entry.
	s, err := newTarGzSink(path, "text", "big-logs", "log", 1)
	require.NoError(t, err)
	writePagesToSink(t, s, records)

	names, content := readAllTarGz(t, path)
	// Multiple chunks → indexed, zero-padded, ordered entry names.
	require.Len(t, names, 2)
	assert.Equal(t, "big-logs.000001.log", names[0])
	assert.Equal(t, "big-logs.000002.log", names[1])

	// Concatenating the parts reconstructs the full log in order.
	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	require.Len(t, lines, 2)
	assert.Equal(t, "cf-1 - gorouter - 10.0.0.1 - info - line one", lines[0])
	assert.Equal(t, "cf-2 - uaa - 10.0.0.2 - error - line two", lines[1])

	// Temp chunk files are cleaned up.
	entries, err := os.ReadDir(filepath.Dir(path))
	require.NoError(t, err)
	for _, e := range entries {
		assert.NotContains(t, e.Name(), ".th-logs-")
	}
}

func TestTarGzSink_Empty(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "empty.tgz")
	s, err := newTarGzSink(path, "text", "empty-logs", "log", logChunkSize)
	require.NoError(t, err)
	require.NoError(t, s.Close())

	// A valid (empty) entry is still present in the archive.
	assert.Equal(t, "", readTarGzEntry(t, path, "empty-logs.log"))
}

func TestTarGzSink_BadPath(t *testing.T) {
	t.Parallel()

	_, err := newTarGzSink(filepath.Join(t.TempDir(), "nope", "out.tgz"), "text", "x", "log", logChunkSize)
	assert.Error(t, err)
}

func TestPlainSink_Empty(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "empty.log")
	s, err := newPlainSink(path, "text")
	require.NoError(t, err)
	require.NoError(t, s.Close())

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, int64(0), info.Size())
}

func TestPlainSink_BadPath(t *testing.T) {
	t.Parallel()

	_, err := newPlainSink(filepath.Join(t.TempDir(), "nonexistent-dir", "out.log"), "text")
	assert.Error(t, err)
}
