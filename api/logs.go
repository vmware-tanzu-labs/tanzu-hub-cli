package api

import (
	"context"
	"fmt"
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
	SortOrder   string          `json:"sortOrder,omitempty"`
	Aggregation *LogAggregation `json:"aggregation,omitempty"`
}

type LogAggregation struct {
	Field string `json:"field,omitempty"`
	Type  string `json:"type"`
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

// logsPageSize is the number of records requested per page. The server caps
// each response at this many records, so larger result sets are retrieved by
// following the page cursor.
const logsPageSize = 1000

// logPage holds a single page of results from queryLogs.
type logPage struct {
	records     []LogRecord
	hasNextPage bool
	endCursor   string
}

// QueryLogs fetches log records for the given entity ID using the supplied
// LogInput, following the page cursor until all matching records are
// retrieved. maxRecords bounds the total number of records returned; a value
// of zero or less means fetch every record in the time window. Any supplied
// progress callbacks are invoked after each page with the cumulative number of
// records fetched so far.
func (g *GraphQLClient) QueryLogs(ctx context.Context, entityID string, input LogInput, maxRecords int, progress ...func(fetched int)) (*LogQueryResult, error) {
	var all []LogRecord
	after := ""

	for {
		pageSize := logsPageSize
		if maxRecords > 0 {
			remaining := maxRecords - len(all)
			if remaining <= 0 {
				break
			}
			if remaining < pageSize {
				pageSize = remaining
			}
		}

		page, err := g.queryLogsPage(ctx, entityID, input, pageSize, after)
		if err != nil {
			return nil, err
		}

		all = append(all, page.records...)

		for _, fn := range progress {
			fn(len(all))
		}

		if !page.hasNextPage || page.endCursor == "" || len(page.records) == 0 {
			break
		}
		after = page.endCursor
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
