package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- GetFoundationEntityID tests ---

const mockFoundationEntityResponse = `{
	"data": {
		"entityQuery": {
			"typed": {
				"tanzu": {
					"tas": {
						"foundation": {
							"query": {
								"entities": [
									{
										"entityId": "vrn/provider:TAS/instance:p-bosh-abc/Foundation:p-bosh-abc",
										"entityName": "ops.eval.lab"
									}
								]
							}
						}
					}
				}
			}
		}
	}
}`

func TestGetFoundationEntityID(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == TokenEndpoint {
			mockAuthEndpoint(w)
			return
		}
		assert.Equal(t, "Bearer mock-token-123", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(mockFoundationEntityResponse))
	}))
	defer server.Close()

	client, err := GetAccessToken("user", "pass", server.URL, true)
	require.NoError(t, err)

	gql := NewGraphQLClient(client, server.URL, true)
	id, err := gql.GetFoundationEntityID(context.Background(), "ops.eval.lab")
	require.NoError(t, err)
	assert.Equal(t, "vrn/provider:TAS/instance:p-bosh-abc/Foundation:p-bosh-abc", id)
}

// queryString extracts the GraphQL "query" field from a request body so tests
// can route responses based on which query is being executed.
func queryString(r *http.Request) string {
	var body struct {
		Query string `json:"query"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	return body.Query
}

func TestGetFoundationEntityID_NotFound(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == TokenEndpoint {
			mockAuthEndpoint(w)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		q := queryString(r)
		switch {
		case strings.Contains(q, "queryManagementEndpointCollectors"):
			// Fallback: no management endpoint matches the name either.
			_, _ = w.Write([]byte(`{"data":{"managementEndpointQuery":{"queryManagementEndpointCollectors":{"managementEndpointCollectors":[]}}}}`))
		default:
			// Typed entity-name lookup: no match.
			_, _ = w.Write([]byte(`{"data":{"entityQuery":{"typed":{"tanzu":{"tas":{"foundation":{"query":{"entities":[]}}}}}}}}`))
		}
	}))
	defer server.Close()

	client, err := GetAccessToken("user", "pass", server.URL, true)
	require.NoError(t, err)

	gql := NewGraphQLClient(client, server.URL, true)
	id, err := gql.GetFoundationEntityID(context.Background(), "missing")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no foundation found")
	assert.Empty(t, id)
}

func TestGetFoundationEntityID_ManagementEndpointFallback(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == TokenEndpoint {
			mockAuthEndpoint(w)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		q := queryString(r)
		switch {
		case strings.Contains(q, "foundation{query(entityName"):
			// Entity-name lookup misses, forcing the fallback path.
			_, _ = w.Write([]byte(`{"data":{"entityQuery":{"typed":{"tanzu":{"tas":{"foundation":{"query":{"entities":[]}}}}}}}}`))
		case strings.Contains(q, "queryManagementEndpointCollectors"):
			_, _ = w.Write([]byte(`{"data":{"managementEndpointQuery":{"queryManagementEndpointCollectors":{"managementEndpointCollectors":[{"managementEndpoint":{"managementEndpointId":"me-123"}}]}}}}`))
		case strings.Contains(q, "queryEntities"):
			// Verify the endpoint ID is forwarded to the entity query.
			assert.Contains(t, q, "managementEndpointId")
			_, _ = w.Write([]byte(`{"data":{"entityQuery":{"queryEntities":{"entities":[{"entityId":"vrn/foundation-xyz"}]}}}}`))
		default:
			t.Fatalf("unexpected query: %s", q)
		}
	}))
	defer server.Close()

	client, err := GetAccessToken("user", "pass", server.URL, true)
	require.NoError(t, err)

	gql := NewGraphQLClient(client, server.URL, true)
	id, err := gql.GetFoundationEntityID(context.Background(), "hub.eval.lab")
	require.NoError(t, err)
	assert.Equal(t, "vrn/foundation-xyz", id)
}

func TestGetFoundationEntityID_GraphQLError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == TokenEndpoint {
			mockAuthEndpoint(w)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errors":[{"message":"forbidden"}]}`))
	}))
	defer server.Close()

	client, err := GetAccessToken("user", "pass", server.URL, true)
	require.NoError(t, err)

	gql := NewGraphQLClient(client, server.URL, true)
	_, err = gql.GetFoundationEntityID(context.Background(), "ops.eval.lab")
	assert.Error(t, err)
}

// --- QueryLogs tests ---

const mockQueryLogsResponse = `{
	"data": {
		"observabilityQuery": {
			"queryLogs": {
				"pageInfo": {"hasNextPage": false, "endCursor": ""},
				"logRecords": [
					{
						"fields": [
							{"key": "severity", "value": "info"},
							{"key": "text", "value": "first message"},
							{"key": "foundation", "value": "ops.eval.lab"}
						]
					},
					{
						"fields": [
							{"key": "severity", "value": "error"},
							{"key": "text", "value": "second message"}
						]
					}
				]
			}
		}
	}
}`

func TestQueryLogs(t *testing.T) {
	t.Parallel()

	var capturedBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == TokenEndpoint {
			mockAuthEndpoint(w)
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&capturedBody)
		assert.Equal(t, "Bearer mock-token-123", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(mockQueryLogsResponse))
	}))
	defer server.Close()

	client, err := GetAccessToken("user", "pass", server.URL, true)
	require.NoError(t, err)

	gql := NewGraphQLClient(client, server.URL, true)
	result, err := gql.QueryLogs(context.Background(), "entity-1", LogInput{
		Namespace: "logs",
		StartTime: "2026-06-06T13:00:00Z",
		EndTime:   "2026-06-06T13:15:00Z",
		SortOrder: "DESC",
	}, 1000)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, 2, result.Count)
	require.Len(t, result.LogRecords, 2)

	r0 := result.LogRecords[0]
	require.Len(t, r0.Fields, 3)
	assert.Equal(t, "severity", r0.Fields[0].Key)
	assert.Equal(t, "info", r0.Fields[0].Value)
	assert.Equal(t, "text", r0.Fields[1].Key)
	assert.Equal(t, "first message", r0.Fields[1].Value)

	// Verify request variables were sent correctly.
	vars, _ := capturedBody["variables"].(map[string]any)
	require.NotNil(t, vars)
	assert.Equal(t, float64(1000), vars["FIRST"])
	ids, _ := vars["ENTITY_IDS"].([]any)
	require.Len(t, ids, 1)
	assert.Equal(t, "entity-1", ids[0])
	input, _ := vars["INPUT"].(map[string]any)
	require.NotNil(t, input)
	assert.Equal(t, "logs", input["namespace"])
	assert.Equal(t, "DESC", input["sortOrder"])
}

func TestQueryLogs_Empty(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == TokenEndpoint {
			mockAuthEndpoint(w)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": {"observabilityQuery": {"queryLogs": {"pageInfo": {"hasNextPage": false, "endCursor": ""}, "logRecords": []}}}
		}`))
	}))
	defer server.Close()

	client, err := GetAccessToken("user", "pass", server.URL, true)
	require.NoError(t, err)

	gql := NewGraphQLClient(client, server.URL, true)
	result, err := gql.QueryLogs(context.Background(), "entity-1", LogInput{Namespace: "logs"}, 100)
	require.NoError(t, err)
	assert.Equal(t, 0, result.Count)
	assert.Empty(t, result.LogRecords)
}

func TestQueryLogs_Paginates(t *testing.T) {
	t.Parallel()

	// Page 1 advertises a next page via endCursor; page 2 ends the sequence.
	page1 := `{"data":{"observabilityQuery":{"queryLogs":{"pageInfo":{"hasNextPage":true,"endCursor":"cursor-2"},"logRecords":[{"fields":[{"key":"text","value":"a"}]},{"fields":[{"key":"text","value":"b"}]}]}}}}`
	page2 := `{"data":{"observabilityQuery":{"queryLogs":{"pageInfo":{"hasNextPage":false,"endCursor":""},"logRecords":[{"fields":[{"key":"text","value":"c"}]}]}}}}`

	var afterValues []any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == TokenEndpoint {
			mockAuthEndpoint(w)
			return
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		vars, _ := body["variables"].(map[string]any)
		afterValues = append(afterValues, vars["AFTER"])

		w.Header().Set("Content-Type", "application/json")
		if vars["AFTER"] == "cursor-2" {
			_, _ = w.Write([]byte(page2))
		} else {
			_, _ = w.Write([]byte(page1))
		}
	}))
	defer server.Close()

	client, err := GetAccessToken("user", "pass", server.URL, true)
	require.NoError(t, err)

	gql := NewGraphQLClient(client, server.URL, true)
	var progressUpdates []int
	result, err := gql.QueryLogs(context.Background(), "entity-1", LogInput{Namespace: "logs"}, 0, func(fetched int) {
		progressUpdates = append(progressUpdates, fetched)
	})
	require.NoError(t, err)

	// All records from both pages are concatenated in order.
	require.Len(t, result.LogRecords, 3)
	assert.Equal(t, 3, result.Count)
	assert.Equal(t, "a", result.LogRecords[0].Fields[0].Value)
	assert.Equal(t, "b", result.LogRecords[1].Fields[0].Value)
	assert.Equal(t, "c", result.LogRecords[2].Fields[0].Value)

	// First request omits the cursor; second forwards the page-1 endCursor.
	require.Len(t, afterValues, 2)
	assert.Nil(t, afterValues[0])
	assert.Equal(t, "cursor-2", afterValues[1])

	// Progress fires once per page with the cumulative record count.
	assert.Equal(t, []int{2, 3}, progressUpdates)
}

func TestQueryLogs_RespectsMaxRecords(t *testing.T) {
	t.Parallel()

	// The server always advertises more pages and honors the requested page
	// size, but maxRecords caps the total fetched.
	var requestedFirst []any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == TokenEndpoint {
			mockAuthEndpoint(w)
			return
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		vars, _ := body["variables"].(map[string]any)
		first := int(vars["FIRST"].(float64))
		requestedFirst = append(requestedFirst, vars["FIRST"])

		records := make([]string, 0, first)
		for i := 0; i < first; i++ {
			records = append(records, `{"fields":[{"key":"text","value":"x"}]}`)
		}
		body2 := `{"data":{"observabilityQuery":{"queryLogs":{"pageInfo":{"hasNextPage":true,"endCursor":"next"},"logRecords":[` +
			strings.Join(records, ",") + `]}}}}`
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body2))
	}))
	defer server.Close()

	client, err := GetAccessToken("user", "pass", server.URL, true)
	require.NoError(t, err)

	gql := NewGraphQLClient(client, server.URL, true)
	result, err := gql.QueryLogs(context.Background(), "entity-1", LogInput{Namespace: "logs"}, 3)
	require.NoError(t, err)

	// Capped at exactly 3 records even though the server keeps offering more.
	require.Len(t, result.LogRecords, 3)
	assert.Equal(t, 3, result.Count)

	// First page requests min(pageSize, 3) = 3; that already satisfies the cap,
	// so no further request is made.
	require.Len(t, requestedFirst, 1)
	assert.Equal(t, float64(3), requestedFirst[0])
}

func TestQueryLogs_GraphQLError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == TokenEndpoint {
			mockAuthEndpoint(w)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errors":[{"message":"BAD_REQUEST"}]}`))
	}))
	defer server.Close()

	client, err := GetAccessToken("user", "pass", server.URL, true)
	require.NoError(t, err)

	gql := NewGraphQLClient(client, server.URL, true)
	result, err := gql.QueryLogs(context.Background(), "entity-1", LogInput{Namespace: "logs"}, 100)
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestGetLogCount(t *testing.T) {
	t.Parallel()

	var capturedInput map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == TokenEndpoint {
			mockAuthEndpoint(w)
			return
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		vars, _ := body["variables"].(map[string]any)
		capturedInput, _ = vars["INPUT"].(map[string]any)

		w.Header().Set("Content-Type", "application/json")
		// COUNT aggregation returns a single record with a "count" field.
		_, _ = w.Write([]byte(`{"data":{"observabilityQuery":{"queryLogs":{"pageInfo":{"hasNextPage":false,"endCursor":""},"logRecords":[{"fields":[{"key":"count","value":"157298"}]}]}}}}`))
	}))
	defer server.Close()

	client, err := GetAccessToken("user", "pass", server.URL, true)
	require.NoError(t, err)

	gql := NewGraphQLClient(client, server.URL, true)
	count, err := gql.GetLogCount(context.Background(), "entity-1", LogInput{
		Namespace: "logs",
		SortOrder: "DESC",
	})
	require.NoError(t, err)
	assert.Equal(t, 157298, count)

	// The count query sets a COUNT aggregation and drops the (irrelevant) sort.
	require.NotNil(t, capturedInput)
	agg, _ := capturedInput["aggregation"].(map[string]any)
	require.NotNil(t, agg)
	assert.Equal(t, "COUNT", agg["type"])
	_, hasSort := capturedInput["sortOrder"]
	assert.False(t, hasSort)
}

func TestGetLogCount_NoCountField(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == TokenEndpoint {
			mockAuthEndpoint(w)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"observabilityQuery":{"queryLogs":{"pageInfo":{"hasNextPage":false,"endCursor":""},"logRecords":[]}}}}`))
	}))
	defer server.Close()

	client, err := GetAccessToken("user", "pass", server.URL, true)
	require.NoError(t, err)

	gql := NewGraphQLClient(client, server.URL, true)
	count, err := gql.GetLogCount(context.Background(), "entity-1", LogInput{Namespace: "logs"})
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestGetLogCount_GraphQLError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == TokenEndpoint {
			mockAuthEndpoint(w)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errors":[{"message":"BAD_REQUEST"}]}`))
	}))
	defer server.Close()

	client, err := GetAccessToken("user", "pass", server.URL, true)
	require.NoError(t, err)

	gql := NewGraphQLClient(client, server.URL, true)
	_, err = gql.GetLogCount(context.Background(), "entity-1", LogInput{Namespace: "logs"})
	assert.Error(t, err)
}

func TestGetAppNames(t *testing.T) {
	t.Parallel()

	var capturedInput map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == TokenEndpoint {
			mockAuthEndpoint(w)
			return
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		vars, _ := body["variables"].(map[string]any)
		capturedInput, _ = vars["INPUT"].(map[string]any)

		w.Header().Set("Content-Type", "application/json")
		// Grouped COUNT returns one record per app name (unsorted, with dupes).
		_, _ = w.Write([]byte(`{"data":{"observabilityQuery":{"queryLogs":{"pageInfo":{"hasNextPage":false,"endCursor":""},"logRecords":[
			{"fields":[{"key":"appname","value":"gorouter"},{"key":"count","value":"500"}]},
			{"fields":[{"key":"appname","value":"bbs"},{"key":"count","value":"300"}]},
			{"fields":[{"key":"appname","value":"gorouter"},{"key":"count","value":"500"}]},
			{"fields":[{"key":"appname","value":"rep"},{"key":"count","value":"100"}]}
		]}}}}`))
	}))
	defer server.Close()

	client, err := GetAccessToken("user", "pass", server.URL, true)
	require.NoError(t, err)

	gql := NewGraphQLClient(client, server.URL, true)
	names, err := gql.GetAppNames(context.Background(), "entity-1", "2026-06-02T00:00:00Z", "2026-06-09T00:00:00Z")
	require.NoError(t, err)

	// Sorted by name and de-duplicated, with each name's count.
	assert.Equal(t, []AppNameCount{
		{Name: "bbs", Count: 300},
		{Name: "gorouter", Count: 500},
		{Name: "rep", Count: 100},
	}, names)

	// The query groups a COUNT aggregation on appname over the observability
	// namespace using a queryTime range.
	require.NotNil(t, capturedInput)
	assert.Equal(t, "Observability", capturedInput["namespace"])
	assert.Equal(t, []any{"appname"}, capturedInput["groupBy"])
	agg, _ := capturedInput["aggregation"].(map[string]any)
	require.NotNil(t, agg)
	assert.Equal(t, "COUNT", agg["type"])
	qt, _ := capturedInput["queryTime"].(map[string]any)
	require.NotNil(t, qt)
	assert.Equal(t, "2026-06-02T00:00:00Z", qt["startTime"])
	assert.Equal(t, "2026-06-09T00:00:00Z", qt["endTime"])
}

func TestGetAppNames_Empty(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == TokenEndpoint {
			mockAuthEndpoint(w)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"observabilityQuery":{"queryLogs":{"pageInfo":{"hasNextPage":false,"endCursor":""},"logRecords":[]}}}}`))
	}))
	defer server.Close()

	client, err := GetAccessToken("user", "pass", server.URL, true)
	require.NoError(t, err)

	gql := NewGraphQLClient(client, server.URL, true)
	names, err := gql.GetAppNames(context.Background(), "entity-1", "2026-06-02T00:00:00Z", "2026-06-09T00:00:00Z")
	require.NoError(t, err)
	assert.Empty(t, names)
}

func TestGetAppNames_GraphQLError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == TokenEndpoint {
			mockAuthEndpoint(w)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errors":[{"message":"BAD_REQUEST"}]}`))
	}))
	defer server.Close()

	client, err := GetAccessToken("user", "pass", server.URL, true)
	require.NoError(t, err)

	gql := NewGraphQLClient(client, server.URL, true)
	_, err = gql.GetAppNames(context.Background(), "entity-1", "2026-06-02T00:00:00Z", "2026-06-09T00:00:00Z")
	assert.Error(t, err)
}

func TestSeverityFilter(t *testing.T) {
	t.Parallel()

	// No severities → no filter.
	assert.Nil(t, SeverityFilter(nil))

	// Single severity → a leaf EQ filter.
	single := SeverityFilter([]string{"error"})
	require.NotNil(t, single)
	assert.Equal(t, "severity", single.Field)
	assert.Equal(t, "EQ", single.Operator)
	assert.Equal(t, []any{"error"}, single.Values)
	assert.Empty(t, single.Or)

	// Multiple severities → an OR of leaf EQ filters.
	multi := SeverityFilter([]string{"error", "info"})
	require.NotNil(t, multi)
	require.Len(t, multi.Or, 2)
	assert.Equal(t, "severity", multi.Or[0].Field)
	assert.Equal(t, "EQ", multi.Or[0].Operator)
	assert.Equal(t, []any{"error"}, multi.Or[0].Values)
	assert.Equal(t, []any{"info"}, multi.Or[1].Values)
}

func TestStreamLogs_SendsSeverityFilter(t *testing.T) {
	t.Parallel()

	var capturedInput map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == TokenEndpoint {
			mockAuthEndpoint(w)
			return
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		vars, _ := body["variables"].(map[string]any)
		capturedInput, _ = vars["INPUT"].(map[string]any)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"observabilityQuery":{"queryLogs":{"pageInfo":{"hasNextPage":false,"endCursor":""},"logRecords":[]}}}}`))
	}))
	defer server.Close()

	client, err := GetAccessToken("user", "pass", server.URL, true)
	require.NoError(t, err)

	gql := NewGraphQLClient(client, server.URL, true)
	_, err = gql.QueryLogs(context.Background(), "entity-1", LogInput{
		Namespace:   "logs",
		QueryFilter: SeverityFilter([]string{"error", "info"}),
	}, 0)
	require.NoError(t, err)

	// The queryFilter is forwarded as an OR of severity EQ leaves.
	require.NotNil(t, capturedInput)
	qf, _ := capturedInput["queryFilter"].(map[string]any)
	require.NotNil(t, qf)
	or, _ := qf["or"].([]any)
	require.Len(t, or, 2)
	leaf0, _ := or[0].(map[string]any)
	assert.Equal(t, "severity", leaf0["field"])
	assert.Equal(t, "EQ", leaf0["operator"])
	assert.Equal(t, []any{"error"}, leaf0["values"])
}

func TestContainsFilter(t *testing.T) {
	t.Parallel()

	// Empty value → no filter.
	assert.Nil(t, ContainsFilter("hostname", ""))

	// A substring → a leaf CONTAINS filter on the named field.
	f := ContainsFilter("hostname", "10.10.0.12")
	require.NotNil(t, f)
	assert.Equal(t, "hostname", f.Field)
	assert.Equal(t, "CONTAINS", f.Operator)
	assert.Equal(t, []any{"10.10.0.12"}, f.Values)
}

func TestAppNameFilter(t *testing.T) {
	t.Parallel()

	// Empty appname → no filter.
	assert.Nil(t, AppNameFilter(""))

	// A substring → a leaf CONTAINS filter.
	f := AppNameFilter("gorouter")
	require.NotNil(t, f)
	assert.Equal(t, "appname", f.Field)
	assert.Equal(t, "CONTAINS", f.Operator)
	assert.Equal(t, []any{"gorouter"}, f.Values)
}

func TestAndFilters(t *testing.T) {
	t.Parallel()

	sev := SeverityFilter([]string{"error"})
	app := AppNameFilter("gorouter")

	// All nil → nil.
	assert.Nil(t, AndFilters(nil, nil))

	// A single non-nil filter is returned as-is (no wrapping AND).
	only := AndFilters(nil, app)
	require.NotNil(t, only)
	assert.Equal(t, "appname", only.Field)
	assert.Empty(t, only.And)

	// Two non-nil filters → an AND composition preserving order.
	both := AndFilters(sev, app)
	require.NotNil(t, both)
	require.Len(t, both.And, 2)
	assert.Equal(t, "severity", both.And[0].Field)
	assert.Equal(t, "appname", both.And[1].Field)
	assert.Equal(t, "CONTAINS", both.And[1].Operator)
}

func TestStreamLogs_SendsAppNameFilter(t *testing.T) {
	t.Parallel()

	var capturedInput map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == TokenEndpoint {
			mockAuthEndpoint(w)
			return
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		vars, _ := body["variables"].(map[string]any)
		capturedInput, _ = vars["INPUT"].(map[string]any)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"observabilityQuery":{"queryLogs":{"pageInfo":{"hasNextPage":false,"endCursor":""},"logRecords":[]}}}}`))
	}))
	defer server.Close()

	client, err := GetAccessToken("user", "pass", server.URL, true)
	require.NoError(t, err)

	gql := NewGraphQLClient(client, server.URL, true)
	_, err = gql.QueryLogs(context.Background(), "entity-1", LogInput{
		Namespace:   "logs",
		QueryFilter: AndFilters(SeverityFilter([]string{"error"}), AppNameFilter("gorouter")),
	}, 0)
	require.NoError(t, err)

	// Both leaves are forwarded under an AND composition.
	require.NotNil(t, capturedInput)
	qf, _ := capturedInput["queryFilter"].(map[string]any)
	require.NotNil(t, qf)
	and, _ := qf["and"].([]any)
	require.Len(t, and, 2)
	sevLeaf, _ := and[0].(map[string]any)
	assert.Equal(t, "severity", sevLeaf["field"])
	assert.Equal(t, "EQ", sevLeaf["operator"])
	appLeaf, _ := and[1].(map[string]any)
	assert.Equal(t, "appname", appLeaf["field"])
	assert.Equal(t, "CONTAINS", appLeaf["operator"])
	assert.Equal(t, []any{"gorouter"}, appLeaf["values"])
}
