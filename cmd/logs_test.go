package cmd

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/laidbackware/tanzu-hub-cli/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

func TestWriteLogsToFile_Text(t *testing.T) {
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
	count, err := writeLogsToFile(path, records, "text")
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	require.Len(t, lines, 2)
	assert.Equal(t, "cf-1 - gorouter - 10.0.0.1 - info - line one", lines[0])
	assert.Equal(t, "cf-2 - uaa - 10.0.0.2 - error - line two", lines[1])
}

func TestWriteLogsToFile_JSON(t *testing.T) {
	t.Parallel()

	records := []api.LogRecord{
		{Fields: []api.LogField{{Key: "text", Value: "line one"}, {Key: "severity", Value: "info"}}},
		{Fields: []api.LogField{{Key: "text", Value: "line two"}, {Key: "severity", Value: "error"}}},
	}

	path := filepath.Join(t.TempDir(), "out.jsonl")
	count, err := writeLogsToFile(path, records, "json")
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()

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

func TestWriteLogsToFile_Empty(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "empty.log")
	count, err := writeLogsToFile(path, nil, "text")
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, int64(0), info.Size())
}

func TestWriteLogsToFile_BadPath(t *testing.T) {
	t.Parallel()

	_, err := writeLogsToFile(filepath.Join(t.TempDir(), "nonexistent-dir", "out.log"), nil, "text")
	assert.Error(t, err)
}
