package api

import (
	"context"
	"fmt"
	"sort"
	"strconv"
)

// EntityId is a named type so the hasura client emits the GraphQL "EntityId"
// scalar for variables (rather than the default "String").
type EntityId string

// foundationEntityTypeConst is the entity type string for a TAS foundation.
const foundationEntityTypeConst = "Tanzu.TAS.Foundation"

// GetFoundationEntityID resolves a name to a TAS foundation entity ID. It first
// treats the name as the foundation entity name; if no match is found it falls
// back to treating the name as a management-endpoint name (as shown by
// "foundation list"). It returns an error if neither lookup succeeds.
func (g *GraphQLClient) GetFoundationEntityID(ctx context.Context, name string) (string, error) {
	id, err := g.foundationEntityIDByName(ctx, name)
	if err != nil {
		return "", err
	}
	if id != "" {
		return id, nil
	}

	id, err = g.foundationEntityIDByManagementEndpoint(ctx, name)
	if err != nil {
		return "", err
	}
	if id != "" {
		return id, nil
	}

	return "", fmt.Errorf("no foundation found with name %q", name)
}

// foundationEntityIDByName looks up a foundation entity by its entity name via
// the typed entity query. It returns an empty string (no error) when no match
// exists so the caller can fall back to other lookups.
func (g *GraphQLClient) foundationEntityIDByName(ctx context.Context, foundationName string) (string, error) {
	var query struct {
		EntityQuery struct {
			Typed struct {
				Tanzu struct {
					Tas struct {
						Foundation struct {
							Query struct {
								Entities []struct {
									EntityID   string `graphql:"entityId"`
									EntityName string `graphql:"entityName"`
								} `graphql:"entities"`
							} `graphql:"query(entityName: $names)"`
						} `graphql:"foundation"`
					} `graphql:"tas"`
				} `graphql:"tanzu"`
			} `graphql:"typed"`
		} `graphql:"entityQuery"`
	}

	vars := map[string]any{"names": []string{foundationName}}
	if err := g.Client.Query(ctx, &query, vars); err != nil {
		return "", err
	}

	entities := query.EntityQuery.Typed.Tanzu.Tas.Foundation.Query.Entities
	if len(entities) == 0 {
		return "", nil
	}

	return entities[0].EntityID, nil
}

// foundationEntityIDByManagementEndpoint resolves a management-endpoint name to
// its foundation entity ID by looking up the endpoint's ID and then querying
// entities scoped to that endpoint. It returns an empty string (no error) when
// no match exists.
func (g *GraphQLClient) foundationEntityIDByManagementEndpoint(ctx context.Context, endpointName string) (string, error) {
	var meQuery struct {
		ManagementEndpointQuery struct {
			QueryManagementEndpointCollectors struct {
				ManagementEndpointCollectors []struct {
					ManagementEndpoint struct {
						ManagementEndpointID string `graphql:"managementEndpointId"`
					} `graphql:"managementEndpoint"`
				} `graphql:"managementEndpointCollectors"`
			} `graphql:"queryManagementEndpointCollectors(name: $name)"`
		} `graphql:"managementEndpointQuery"`
	}

	if err := g.Client.Query(ctx, &meQuery, map[string]any{"name": []string{endpointName}}); err != nil {
		return "", err
	}

	collectors := meQuery.ManagementEndpointQuery.QueryManagementEndpointCollectors.ManagementEndpointCollectors
	if len(collectors) == 0 {
		return "", nil
	}
	meID := collectors[0].ManagementEndpoint.ManagementEndpointID
	if meID == "" {
		return "", nil
	}

	var entQuery struct {
		EntityQuery struct {
			QueryEntities struct {
				Entities []struct {
					EntityID string `graphql:"entityId"`
				} `graphql:"entities"`
			} `graphql:"queryEntities(managementEndpointId: $meId, entityType: $etype, first: 1)"`
		} `graphql:"entityQuery"`
	}

	vars := map[string]any{
		"meId":  []string{meID},
		"etype": []string{foundationEntityTypeConst},
	}
	if err := g.Client.Query(ctx, &entQuery, vars); err != nil {
		return "", err
	}

	entities := entQuery.EntityQuery.QueryEntities.Entities
	if len(entities) == 0 {
		return "", nil
	}

	return entities[0].EntityID, nil
}

// Input type — json tags for hasura variable serialization. The struct name
// must match the GraphQL "LogInput" type.

type LogInput struct {
	Namespace   string          `json:"namespace"`
	StartTime   string          `json:"startTime,omitempty"`
	EndTime     string          `json:"endTime,omitempty"`
	QueryTime   *LogTimeRange   `json:"queryTime,omitempty"`
	GroupBy     []string        `json:"groupBy,omitempty"`
	QueryFilter *LogQueryFilter `json:"queryFilter,omitempty"`
	SortOrder   string          `json:"sortOrder,omitempty"`
	Aggregation *LogAggregation `json:"aggregation,omitempty"`
}

type LogTimeRange struct {
	StartTime string `json:"startTime,omitempty"`
	EndTime   string `json:"endTime,omitempty"`
}

type LogAggregation struct {
	Field string `json:"field,omitempty"`
	Type  string `json:"type"`
}

// LogQueryFilter mirrors the GraphQL QueryFilter input: a leaf
// (field/operator/values) or a boolean composition (and/or/not).
type LogQueryFilter struct {
	And      []LogQueryFilter `json:"and,omitempty"`
	Or       []LogQueryFilter `json:"or,omitempty"`
	Not      []LogQueryFilter `json:"not,omitempty"`
	Field    string           `json:"field,omitempty"`
	Operator string           `json:"operator,omitempty"`
	Values   []any            `json:"values,omitempty"`
}

// SeverityFilter builds a QueryFilter matching log records whose "severity"
// field equals any of the given values. It returns nil when no severities are
// supplied (i.e. no filtering).
func SeverityFilter(severities []string) *LogQueryFilter {
	switch len(severities) {
	case 0:
		return nil
	case 1:
		return &LogQueryFilter{Field: "severity", Operator: "EQ", Values: []any{severities[0]}}
	default:
		or := make([]LogQueryFilter, len(severities))
		for i, s := range severities {
			or[i] = LogQueryFilter{Field: "severity", Operator: "EQ", Values: []any{s}}
		}
		return &LogQueryFilter{Or: or}
	}
}

// ContainsFilter builds a QueryFilter matching log records whose named field
// contains the given substring. It returns nil when value is empty (i.e. no
// filtering).
func ContainsFilter(field, value string) *LogQueryFilter {
	if value == "" {
		return nil
	}
	return &LogQueryFilter{Field: field, Operator: "CONTAINS", Values: []any{value}}
}

// AppNameFilter builds a QueryFilter matching log records whose "appname" field
// contains the given substring. It returns nil when appName is empty (i.e. no
// filtering).
func AppNameFilter(appName string) *LogQueryFilter {
	return ContainsFilter("appname", appName)
}

// AndFilters combines the given filters into a single QueryFilter that matches
// only records satisfying all of them. nil filters are ignored. It returns nil
// when no non-nil filters remain, and the filter itself when only one remains.
func AndFilters(filters ...*LogQueryFilter) *LogQueryFilter {
	and := make([]LogQueryFilter, 0, len(filters))
	for _, f := range filters {
		if f != nil {
			and = append(and, *f)
		}
	}
	switch len(and) {
	case 0:
		return nil
	case 1:
		return &and[0]
	default:
		return &LogQueryFilter{And: and}
	}
}

// Response types — graphql tags for hasura client deserialization.

type LogField struct {
	Key   string `graphql:"key"   json:"key"   yaml:"key"`
	Value string `graphql:"value" json:"value" yaml:"value"`
}

type LogRecord struct {
	Fields []LogField `graphql:"fields" json:"fields" yaml:"fields"`
}

type LogQueryResult struct {
	LogRecords []LogRecord
	Count      int
	TotalCount int
}

// logsPageSize is the number of records requested per page. The server enforces
// a hard limit of 2000 records per request (requesting more is rejected with
// "exceeds the permissible limit(Max: 2000 records)"), so we request the maximum
// and retrieve larger result sets by following the page cursor.
const logsPageSize = 2000

// logPage holds a single page of results from queryLogs.
type logPage struct {
	records     []LogRecord
	hasNextPage bool
	endCursor   string
}

// StreamLogs fetches log records for the given entity ID using the supplied
// LogInput, following the page cursor until all matching records are retrieved.
// Each page is handed to onPage as it arrives — records are never accumulated
// in full — making it suitable for very large result sets. maxRecords bounds
// the total number of records fetched; a value of zero or less means fetch
// every record in the time window. Any supplied progress callbacks are invoked
// after each page with the cumulative number of records fetched so far. It
// returns the total number of records fetched.
func (g *GraphQLClient) StreamLogs(ctx context.Context, entityID string, input LogInput, maxRecords int, onPage func(records []LogRecord) error, progress ...func(fetched int)) (int, error) {
	fetched := 0
	after := ""

	for {
		pageSize := logsPageSize
		if maxRecords > 0 {
			remaining := maxRecords - fetched
			if remaining <= 0 {
				break
			}
			if remaining < pageSize {
				pageSize = remaining
			}
		}

		page, err := g.queryLogsPage(ctx, entityID, input, pageSize, after)
		if err != nil {
			return fetched, err
		}

		if len(page.records) > 0 {
			if onPage != nil {
				if err := onPage(page.records); err != nil {
					return fetched, err
				}
			}
			fetched += len(page.records)
			for _, fn := range progress {
				fn(fetched)
			}
		}

		if !page.hasNextPage || page.endCursor == "" || len(page.records) == 0 {
			break
		}
		after = page.endCursor
	}

	return fetched, nil
}

// QueryLogs is a convenience wrapper around StreamLogs that accumulates every
// matching record into a single result. Prefer StreamLogs for large result
// sets where holding all records in memory is undesirable.
func (g *GraphQLClient) QueryLogs(ctx context.Context, entityID string, input LogInput, maxRecords int, progress ...func(fetched int)) (*LogQueryResult, error) {
	var all []LogRecord
	_, err := g.StreamLogs(ctx, entityID, input, maxRecords, func(records []LogRecord) error {
		all = append(all, records...)
		return nil
	}, progress...)
	if err != nil {
		return nil, err
	}

	return &LogQueryResult{
		LogRecords: all,
		Count:      len(all),
		TotalCount: len(all),
	}, nil
}

// GetLogCount returns the exact number of log records matching the supplied
// LogInput by running a COUNT aggregation. The connection's totalCount field is
// always null and count only reflects the page size, so this aggregation is the
// only reliable way to obtain the total ahead of downloading.
func (g *GraphQLClient) GetLogCount(ctx context.Context, entityID string, input LogInput) (int, error) {
	countInput := input
	countInput.SortOrder = ""
	countInput.Aggregation = &LogAggregation{Type: "COUNT"}

	page, err := g.queryLogsPage(ctx, entityID, countInput, 1, "")
	if err != nil {
		return 0, err
	}

	for _, rec := range page.records {
		for _, fld := range rec.Fields {
			if fld.Key == "count" {
				return strconv.Atoi(fld.Value)
			}
		}
	}

	return 0, nil
}

// appNamesNamespace is the log namespace used for observability aggregations.
const appNamesNamespace = "Observability"

// AppNameCount pairs a distinct application name with the number of log records
// it produced in the queried window.
type AppNameCount struct {
	Name  string `json:"name"  yaml:"name"`
	Count int    `json:"count" yaml:"count"`
}

// GetAppNames returns the distinct application names that produced logs for the
// entity within [startTime, endTime], along with each one's log record count,
// sorted by name. It works by grouping a COUNT aggregation on the "appname"
// field, so each result record represents one distinct app name.
func (g *GraphQLClient) GetAppNames(ctx context.Context, entityID, startTime, endTime string) ([]AppNameCount, error) {
	input := LogInput{
		Namespace:   appNamesNamespace,
		QueryTime:   &LogTimeRange{StartTime: startTime, EndTime: endTime},
		GroupBy:     []string{"appname"},
		SortOrder:   "DESC",
		Aggregation: &LogAggregation{Type: "COUNT"},
	}

	page, err := g.queryLogsPage(ctx, entityID, input, logsPageSize, "")
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	var results []AppNameCount
	for _, rec := range page.records {
		fields := make(map[string]string, len(rec.Fields))
		for _, fld := range rec.Fields {
			fields[fld.Key] = fld.Value
		}
		name := fields["appname"]
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		count, _ := strconv.Atoi(fields["count"])
		results = append(results, AppNameCount{Name: name, Count: count})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Name < results[j].Name
	})
	return results, nil
}

// queryLogsPage fetches a single page of log records. When after is empty the
// cursor argument is omitted so the server returns the first page.
func (g *GraphQLClient) queryLogsPage(ctx context.Context, entityID string, input LogInput, first int, after string) (*logPage, error) {
	type queryLogsResult struct {
		LogRecords []LogRecord `graphql:"logRecords"`
		PageInfo   struct {
			HasNextPage bool   `graphql:"hasNextPage"`
			EndCursor   string `graphql:"endCursor"`
		} `graphql:"pageInfo"`
	}

	vars := map[string]any{
		"ENTITY_IDS": []EntityId{EntityId(entityID)},
		"FIRST":      first,
		"INPUT":      input,
	}

	var result queryLogsResult
	if after != "" {
		var query struct {
			ObservabilityQuery struct {
				QueryLogs queryLogsResult `graphql:"queryLogs(entityId: $ENTITY_IDS, first: $FIRST, after: $AFTER, input: $INPUT)"`
			} `graphql:"observabilityQuery"`
		}
		vars["AFTER"] = after
		if err := g.Client.Query(ctx, &query, vars); err != nil {
			return nil, err
		}
		result = query.ObservabilityQuery.QueryLogs
	} else {
		var query struct {
			ObservabilityQuery struct {
				QueryLogs queryLogsResult `graphql:"queryLogs(entityId: $ENTITY_IDS, first: $FIRST, input: $INPUT)"`
			} `graphql:"observabilityQuery"`
		}
		if err := g.Client.Query(ctx, &query, vars); err != nil {
			return nil, err
		}
		result = query.ObservabilityQuery.QueryLogs
	}

	return &logPage{
		records:     result.LogRecords,
		hasNextPage: result.PageInfo.HasNextPage,
		endCursor:   result.PageInfo.EndCursor,
	}, nil
}
