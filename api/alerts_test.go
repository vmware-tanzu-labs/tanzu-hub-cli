package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const mockAlertsResponse = `{
	"data": {
		"hubPolicyQuery": {
			"providers": {
				"observabilityAlert": {
					"queryHubPolicies": {
						"count": 2,
						"totalCount": 2,
						"pageInfo": {
							"hasPreviousPage": false,
							"hasNextPage": false,
							"startCursor": "cursor-start",
							"endCursor": "cursor-end"
						},
						"policies": [
							{
								"__typename": "ObservabilityMetricAlert",
								"nodeVersion": "v1",
								"policyId": "policy-001",
								"policyName": "High CPU Alert",
								"description": "Fires when CPU exceeds threshold",
								"entityType": "FOUNDATION",
								"enabled": true,
								"runbookTemplateId": "runbook-1",
								"dashboardId": "dash-1",
								"dataScopedEntities": {
									"entityIds": ["entity-a", "entity-b"],
									"entityType": "FOUNDATION"
								},
								"entityInfo": {
									"entityId": "entity-a",
									"entityName": "my-foundation",
									"entityType": "FOUNDATION"
								},
								"createdBy": {"userAccount": "alice@example.com"},
								"creationTime": "2025-01-01T00:00:00Z",
								"updatedBy": {"userAccount": "bob@example.com"},
								"lastUpdateTime": "2025-06-01T00:00:00Z",
								"lastRunTime": "2025-06-15T00:00:00Z",
								"policyProvider": "observabilityAlert",
								"source": "tanzu",
								"alertType": "METRIC",
								"alertStatus": "ACTIVE",
								"isCustomAlert": false,
								"rule": {
									"queryString": "sum(rate(cpu_usage[5m]))",
									"queryStringMetadata": "metadata-1",
									"namespace": "default",
									"alertCondition": {
										"includedEntityIds": ["entity-a"],
										"threshold": {
											"alertSeverity": "CRITICAL",
											"thresholdValue": 90.0
										},
										"operator": "GREATER_THAN",
										"tags": [{"key": "env", "value": "prod"}],
										"overrideRules": [
											{
												"labelName": "team",
												"labelValues": ["infra"],
												"ruleName": "infra-override",
												"tags": [{"key": "tier", "value": "1"}],
												"threshold": {
													"alertSeverity": "WARNING",
													"thresholdValue": 80.0
												}
											}
										],
										"triggerWindowDuration": {"duration": 5, "durationUnit": "MINUTES"},
										"resolveWindowDuration": {"duration": 10, "durationUnit": "MINUTES"},
										"alertEvaluationFrequencyDuration": {"duration": 1, "durationUnit": "MINUTES"}
									}
								}
							},
							{
								"__typename": "ObservabilityMetricAlert",
								"nodeVersion": "v2",
								"policyId": "policy-002",
								"policyName": "Memory Alert",
								"description": "",
								"entityType": "APP",
								"enabled": false,
								"runbookTemplateId": "",
								"dashboardId": "",
								"dataScopedEntities": {"entityIds": [], "entityType": "APP"},
								"entityInfo": {"entityId": "", "entityName": "", "entityType": ""},
								"createdBy": {"userAccount": "carol@example.com"},
								"creationTime": "2025-03-01T00:00:00Z",
								"updatedBy": {"userAccount": ""},
								"lastUpdateTime": "",
								"lastRunTime": "",
								"policyProvider": "observabilityAlert",
								"source": "tanzu",
								"alertType": "METRIC",
								"alertStatus": "INACTIVE",
								"isCustomAlert": true,
								"rule": {
									"queryString": "sum(memory_usage)",
									"queryStringMetadata": "",
									"namespace": "",
									"alertCondition": {
										"includedEntityIds": [],
										"threshold": {"alertSeverity": "WARNING", "thresholdValue": 75.0},
										"operator": "GREATER_THAN",
										"tags": [],
										"overrideRules": [],
										"triggerWindowDuration": {"duration": 2, "durationUnit": "MINUTES"},
										"resolveWindowDuration": {"duration": 5, "durationUnit": "MINUTES"},
										"alertEvaluationFrequencyDuration": {"duration": 1, "durationUnit": "MINUTES"}
									}
								}
							}
						]
					}
				}
			}
		}
	}
}`

// --- Pure function tests ---

func TestStripHTML(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, input, want string
	}{
		{"paragraph tags", "<p>Hello world</p>", "Hello world"},
		{"nested tags", "<p>The <b>PAS</b> service is <i>down</i>.</p>", "The PAS service is down."},
		{"no tags", "plain text", "plain text"},
		{"empty string", "", ""},
		{"self-closing tag", "before<br/>after", "beforeafter"},
		{"whitespace trimming", "  <p> padded </p>  ", "padded"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, stripHTML(tc.input))
		})
	}
}

func TestToMinutes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		dur  AlertDuration
		want int
	}{
		{"minutes", AlertDuration{Duration: 5, DurationUnit: "MINUTES"}, 5},
		{"mins alias", AlertDuration{Duration: 10, DurationUnit: "MINS"}, 10},
		{"hours", AlertDuration{Duration: 2, DurationUnit: "HOURS"}, 120},
		{"seconds over 60", AlertDuration{Duration: 180, DurationUnit: "SECONDS"}, 3},
		{"seconds under 60", AlertDuration{Duration: 30, DurationUnit: "SECONDS"}, 1},
		{"empty unit defaults to minutes", AlertDuration{Duration: 7, DurationUnit: ""}, 7},
		{"lowercase minutes", AlertDuration{Duration: 3, DurationUnit: "minutes"}, 3},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, toMinutes(tc.dur))
		})
	}
}

func newTestAlert() ObservabilityMetricAlert {
	return ObservabilityMetricAlert{
		NodeVersion:       "MQ==",
		PolicyID:          "policy-123",
		PolicyName:        "Test Alert",
		Description:       "<p>Alert description</p>",
		EntityType:        "FOUNDATION",
		Enabled:           true,
		RunbookTemplateID: "runbook-1",
		DashboardID:       "dash-1",
		EntityInfo: AlertEntityInfo{
			EntityID:   "entity-1",
			EntityName: "my-entity",
			EntityType: "FOUNDATION",
		},
		PolicyProvider: "observabilityAlert",
		Source:         "TP_CF",
		AlertType:      "METRIC",
		AlertStatus:    "ACTIVE",
		IsCustomAlert:  true,
		ThresholdType:  "STATIC",
		Rule: MetricAlertRule{
			QueryString:         "system_healthy{job='exporter'}",
			QueryStringMetadata: "meta",
			Namespace:           "Observability",
			AlertCondition: MetricAlertCondition{
				IncludedEntityIDs: []string{"entity-1"},
				Threshold: AlertThresholdCondition{
					AlertSeverity:  "CRITICAL",
					ThresholdValue: 1.0,
				},
				Operator: "LT",
				Tags:     []AlertTag{{Key: "env", Value: "prod"}},
				OverrideRules: []AlertOverrideRule{
					{
						LabelName:   "team",
						LabelValues: []string{"infra"},
						RuleName:    "infra-override",
						Tags:        []AlertTag{{Key: "tier", Value: "1"}},
						Threshold: AlertThresholdCondition{
							AlertSeverity:  "WARNING",
							ThresholdValue: 0.5,
						},
					},
				},
				TriggerWindowDuration:            AlertDuration{Duration: 5, DurationUnit: "MINUTES"},
				ResolveWindowDuration:            AlertDuration{Duration: 10, DurationUnit: "MINUTES"},
				AlertEvaluationFrequencyDuration: AlertDuration{Duration: 1, DurationUnit: "MINUTES"},
			},
		},
	}
}

func TestBuildAlertRuleInput(t *testing.T) {
	t.Parallel()
	a := newTestAlert()
	rule := buildAlertRuleInput(a)

	assert.Equal(t, "system_healthy{job='exporter'}", rule.QueryString)
	assert.Equal(t, "meta", rule.QueryStringMetadata)
	assert.Equal(t, "Observability", rule.Namespace)
	assert.Equal(t, "STATIC", rule.ThresholdType)

	cond := rule.AlertConditionInput
	assert.Equal(t, "LT", cond.Operator)
	assert.Equal(t, 5, cond.TriggerWindowInMins)
	assert.Equal(t, 10, cond.ResolveWindowInMins)
	assert.Equal(t, 1, cond.AlertEvaluationFrequencyInMins)
	assert.Equal(t, "CRITICAL", cond.Threshold.AlertSeverity)
	assert.Equal(t, 1.0, cond.Threshold.ThresholdValue)
	assert.Equal(t, []string{"entity-1"}, cond.IncludedEntityIDs)

	require.Len(t, cond.Tags, 1)
	assert.Equal(t, "env", cond.Tags[0].Key)
	assert.Equal(t, "prod", cond.Tags[0].Value)

	require.Len(t, cond.OverrideRules, 1)
	or := cond.OverrideRules[0]
	assert.Equal(t, "team", or.LabelName)
	assert.Equal(t, []string{"infra"}, or.LabelValues)
	assert.Equal(t, "infra-override", or.RuleName)
	assert.Equal(t, "WARNING", or.Threshold.AlertSeverity)
	assert.Equal(t, 0.5, or.Threshold.ThresholdValue)
	require.Len(t, or.Tags, 1)
	assert.Equal(t, "tier", or.Tags[0].Key)
}

func TestBuildAlertRuleInput_DefaultThresholdType(t *testing.T) {
	t.Parallel()
	a := newTestAlert()
	a.ThresholdType = ""
	rule := buildAlertRuleInput(a)
	assert.Equal(t, "STATIC", rule.ThresholdType)
}

func TestAlertToCreateInput(t *testing.T) {
	t.Parallel()
	a := newTestAlert()
	input := AlertToCreateInput(a)

	assert.Equal(t, "Alert description", input.Description)
	assert.Equal(t, "entity-1", input.EntityID)
	assert.Equal(t, "FOUNDATION", input.EntityType)
	assert.Equal(t, "Test Alert", input.Name)
	assert.Equal(t, "runbook-1", input.RunbookTemplateID)
	assert.Equal(t, "TP_CF", input.Source)
	assert.Equal(t, "system_healthy{job='exporter'}", input.Rule.QueryString)
}

func TestAlertToUpdateInput(t *testing.T) {
	t.Parallel()
	a := newTestAlert()
	input := AlertToUpdateInput(a)

	assert.Equal(t, "Alert description", input.Description)
	assert.Equal(t, "entity-1", input.EntityID)
	assert.Equal(t, "FOUNDATION", input.EntityType)
	assert.Equal(t, "Test Alert", input.Name)
	assert.Equal(t, "MQ==", input.NodeVersion)
	assert.Equal(t, "policy-123", input.PolicyID)
	assert.Equal(t, "runbook-1", input.RunbookTemplateID)
	assert.Equal(t, "TP_CF", input.Source)
	assert.Equal(t, "system_healthy{job='exporter'}", input.Rule.QueryString)
}

// --- GraphQL client tests ---

func TestQueryObservabilityAlerts(t *testing.T) {
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
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(mockAlertsResponse))
	}))
	defer server.Close()

	client, err := GetAccessToken("user", "pass", server.URL, true)
	require.NoError(t, err)

	gql := NewGraphQLClient(client, server.URL, true)
	result, err := gql.QueryObservabilityAlerts(
		context.Background(),
		ObservabilityAlertFilterInput{AlertType: "METRIC"},
		[]QuerySort{{Field: "creationTime", Order: "DESC"}},
		10, "", "",
	)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, 2, result.Count)
	assert.Equal(t, 2, result.TotalCount)
	assert.False(t, result.PageInfo.HasPreviousPage)
	assert.False(t, result.PageInfo.HasNextPage)
	assert.Equal(t, "cursor-start", result.PageInfo.StartCursor)
	assert.Equal(t, "cursor-end", result.PageInfo.EndCursor)

	require.Len(t, result.Alerts, 2)

	a0 := result.Alerts[0]
	assert.Equal(t, "policy-001", a0.PolicyID)
	assert.Equal(t, "High CPU Alert", a0.PolicyName)
	assert.Equal(t, "Fires when CPU exceeds threshold", a0.Description)
	assert.Equal(t, "FOUNDATION", a0.EntityType)
	assert.True(t, a0.Enabled)
	assert.Equal(t, "runbook-1", a0.RunbookTemplateID)
	assert.Equal(t, "dash-1", a0.DashboardID)
	assert.Equal(t, "METRIC", a0.AlertType)
	assert.Equal(t, "ACTIVE", a0.AlertStatus)
	assert.False(t, a0.IsCustomAlert)
	assert.Equal(t, "alice@example.com", a0.CreatedBy.UserAccount)
	assert.Equal(t, "bob@example.com", a0.UpdatedBy.UserAccount)
	assert.Equal(t, "2025-01-01T00:00:00Z", a0.CreationTime)
	assert.Equal(t, "entity-a", a0.EntityInfo.EntityID)
	assert.Equal(t, "my-foundation", a0.EntityInfo.EntityName)
	assert.Equal(t, []string{"entity-a", "entity-b"}, a0.DataScopedEntities.EntityIDs)
	assert.Equal(t, "FOUNDATION", a0.DataScopedEntities.EntityType)

	rule := a0.Rule
	assert.Equal(t, "sum(rate(cpu_usage[5m]))", rule.QueryString)
	assert.Equal(t, "metadata-1", rule.QueryStringMetadata)
	assert.Equal(t, "default", rule.Namespace)

	cond := rule.AlertCondition
	assert.Equal(t, "GREATER_THAN", cond.Operator)
	assert.Equal(t, "CRITICAL", cond.Threshold.AlertSeverity)
	assert.Equal(t, 90.0, cond.Threshold.ThresholdValue)
	assert.Equal(t, []string{"entity-a"}, cond.IncludedEntityIDs)
	require.Len(t, cond.Tags, 1)
	assert.Equal(t, "env", cond.Tags[0].Key)
	assert.Equal(t, "prod", cond.Tags[0].Value)
	assert.Equal(t, 5, cond.TriggerWindowDuration.Duration)
	assert.Equal(t, "MINUTES", cond.TriggerWindowDuration.DurationUnit)
	assert.Equal(t, 10, cond.ResolveWindowDuration.Duration)
	assert.Equal(t, 1, cond.AlertEvaluationFrequencyDuration.Duration)

	require.Len(t, cond.OverrideRules, 1)
	or0 := cond.OverrideRules[0]
	assert.Equal(t, "team", or0.LabelName)
	assert.Equal(t, []string{"infra"}, or0.LabelValues)
	assert.Equal(t, "infra-override", or0.RuleName)
	assert.Equal(t, "WARNING", or0.Threshold.AlertSeverity)
	assert.Equal(t, 80.0, or0.Threshold.ThresholdValue)

	a1 := result.Alerts[1]
	assert.Equal(t, "policy-002", a1.PolicyID)
	assert.False(t, a1.Enabled)
	assert.True(t, a1.IsCustomAlert)
	assert.Equal(t, "INACTIVE", a1.AlertStatus)
	assert.Equal(t, "carol@example.com", a1.CreatedBy.UserAccount)
	assert.Empty(t, a1.Rule.AlertCondition.OverrideRules)
}

func TestQueryObservabilityAlerts_GraphQLError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == TokenEndpoint {
			mockAuthEndpoint(w)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"errors":[{"message":"unauthorized"}]}`))
	}))
	defer server.Close()

	client, err := GetAccessToken("user", "pass", server.URL, true)
	require.NoError(t, err)

	gql := NewGraphQLClient(client, server.URL, true)
	result, err := gql.QueryObservabilityAlerts(
		context.Background(),
		ObservabilityAlertFilterInput{},
		nil, 10, "", "",
	)
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestQueryObservabilityAlerts_AfterCursor(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == TokenEndpoint {
			mockAuthEndpoint(w)
			return
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		vars, _ := body["variables"].(map[string]any)
		assert.Equal(t, "cursor-abc", vars["AFTER"])
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(mockAlertsResponse))
	}))
	defer server.Close()

	client, err := GetAccessToken("user", "pass", server.URL, true)
	require.NoError(t, err)

	gql := NewGraphQLClient(client, server.URL, true)
	result, err := gql.QueryObservabilityAlerts(
		context.Background(),
		ObservabilityAlertFilterInput{AlertType: "METRIC"},
		[]QuerySort{{Field: "creationTime", Order: "DESC"}},
		10, "cursor-abc", "",
	)
	require.NoError(t, err)
	assert.Len(t, result.Alerts, 2)
}

func TestQueryObservabilityAlerts_BeforeCursor(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == TokenEndpoint {
			mockAuthEndpoint(w)
			return
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		vars, _ := body["variables"].(map[string]any)
		assert.Equal(t, "cursor-xyz", vars["BEFORE"])
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(mockAlertsResponse))
	}))
	defer server.Close()

	client, err := GetAccessToken("user", "pass", server.URL, true)
	require.NoError(t, err)

	gql := NewGraphQLClient(client, server.URL, true)
	result, err := gql.QueryObservabilityAlerts(
		context.Background(),
		ObservabilityAlertFilterInput{AlertType: "METRIC"},
		[]QuerySort{{Field: "creationTime", Order: "DESC"}},
		10, "", "cursor-xyz",
	)
	require.NoError(t, err)
	assert.Len(t, result.Alerts, 2)
}

func TestQueryObservabilityAlerts_Empty(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == TokenEndpoint {
			mockAuthEndpoint(w)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"data": {
				"hubPolicyQuery": {
					"providers": {
						"observabilityAlert": {
							"queryHubPolicies": {
								"count": 0,
								"totalCount": 0,
								"pageInfo": {
									"hasPreviousPage": false,
									"hasNextPage": false,
									"startCursor": "",
									"endCursor": ""
								},
								"policies": []
							}
						}
					}
				}
			}
		}`))
	}))
	defer server.Close()

	client, err := GetAccessToken("user", "pass", server.URL, true)
	require.NoError(t, err)

	gql := NewGraphQLClient(client, server.URL, true)
	result, err := gql.QueryObservabilityAlerts(
		context.Background(),
		ObservabilityAlertFilterInput{AlertType: "METRIC"},
		[]QuerySort{{Field: "creationTime", Order: "DESC"}},
		10, "", "",
	)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 0, result.Count)
	assert.Empty(t, result.Alerts)
	assert.False(t, result.PageInfo.HasNextPage)
}

// --- Mutation tests ---

const mockCreateAlertResponse = `{
	"data": {
		"hubPolicyMutation": {
			"providers": {
				"observabilityAlert": {
					"createMetricAlert": {
						"policyId": "new-policy-001",
						"policyName": "New Alert"
					}
				}
			}
		}
	}
}`

func TestCreateMetricAlert(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == TokenEndpoint {
			mockAuthEndpoint(w)
			return
		}
		assert.Equal(t, "Bearer mock-token-123", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(mockCreateAlertResponse))
	}))
	defer server.Close()

	client, err := GetAccessToken("user", "pass", server.URL, true)
	require.NoError(t, err)

	gql := NewGraphQLClient(client, server.URL, true)
	input := AlertToCreateInput(newTestAlert())
	result, err := gql.CreateMetricAlert(context.Background(), input)
	require.NoError(t, err)
	assert.Equal(t, "new-policy-001", result.PolicyID)
	assert.Equal(t, "New Alert", result.PolicyName)
}

func TestCreateMetricAlert_GraphQLError(t *testing.T) {
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
	result, err := gql.CreateMetricAlert(context.Background(), ObservabilityMetricAlertCreateInput{})
	assert.Error(t, err)
	assert.Nil(t, result)
}

const mockUpdateAlertResponse = `{
	"data": {
		"hubPolicyMutation": {
			"providers": {
				"observabilityAlert": {
					"updateMetricAlert": {
						"policyId": "policy-123",
						"policyName": "Updated Alert"
					}
				}
			}
		}
	}
}`

func TestUpdateMetricAlert(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == TokenEndpoint {
			mockAuthEndpoint(w)
			return
		}
		assert.Equal(t, "Bearer mock-token-123", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(mockUpdateAlertResponse))
	}))
	defer server.Close()

	client, err := GetAccessToken("user", "pass", server.URL, true)
	require.NoError(t, err)

	gql := NewGraphQLClient(client, server.URL, true)
	input := AlertToUpdateInput(newTestAlert())
	result, err := gql.UpdateMetricAlert(context.Background(), input)
	require.NoError(t, err)
	assert.Equal(t, "policy-123", result.PolicyID)
	assert.Equal(t, "Updated Alert", result.PolicyName)
}

func TestUpdateMetricAlert_GraphQLError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == TokenEndpoint {
			mockAuthEndpoint(w)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errors":[{"message":"NOT_FOUND"}]}`))
	}))
	defer server.Close()

	client, err := GetAccessToken("user", "pass", server.URL, true)
	require.NoError(t, err)

	gql := NewGraphQLClient(client, server.URL, true)
	result, err := gql.UpdateMetricAlert(context.Background(), ObservabilityMetricAlertUpdateInput{})
	assert.Error(t, err)
	assert.Nil(t, result)
}

const mockDeleteAlertResponse = `{
	"data": {
		"hubPolicyMutation": {
			"providers": {
				"observabilityAlert": {
					"deleteAlerts": {
						"failureCount": 0,
						"successCount": 1
					}
				}
			}
		}
	}
}`

func TestDeleteAlerts(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == TokenEndpoint {
			mockAuthEndpoint(w)
			return
		}
		assert.Equal(t, "Bearer mock-token-123", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(mockDeleteAlertResponse))
	}))
	defer server.Close()

	client, err := GetAccessToken("user", "pass", server.URL, true)
	require.NoError(t, err)

	gql := NewGraphQLClient(client, server.URL, true)
	result, err := gql.DeleteAlerts(context.Background(), []HubPolicyIdInput{
		{PolicyID: "policy-123", NodeVersion: "MQ=="},
	})
	require.NoError(t, err)
	assert.Equal(t, 1, result.SuccessCount)
	assert.Equal(t, 0, result.FailureCount)
}

func TestDeleteAlerts_NullResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == TokenEndpoint {
			mockAuthEndpoint(w)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": {
				"hubPolicyMutation": {
					"providers": {
						"observabilityAlert": {
							"deleteAlerts": null
						}
					}
				}
			}
		}`))
	}))
	defer server.Close()

	client, err := GetAccessToken("user", "pass", server.URL, true)
	require.NoError(t, err)

	gql := NewGraphQLClient(client, server.URL, true)
	result, err := gql.DeleteAlerts(context.Background(), []HubPolicyIdInput{
		{PolicyID: "policy-123", NodeVersion: "MQ=="},
	})
	require.NoError(t, err)
	assert.Equal(t, 0, result.SuccessCount)
	assert.Equal(t, 0, result.FailureCount)
}

func TestDeleteAlerts_GraphQLError(t *testing.T) {
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
	result, err := gql.DeleteAlerts(context.Background(), []HubPolicyIdInput{
		{PolicyID: "policy-123", NodeVersion: "MQ=="},
	})
	assert.Error(t, err)
	assert.Nil(t, result)
}

// --- UpdateAlertStatus tests ---

const mockUpdateAlertStatusResponse = `{
	"data": {
		"hubPolicyMutation": {
			"providers": {
				"observabilityAlert": {
					"updateAlertStatus": {
						"failureCount": 0,
						"successCount": 1
					}
				}
			}
		}
	}
}`

func TestUpdateAlertStatus(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == TokenEndpoint {
			mockAuthEndpoint(w)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(mockUpdateAlertStatusResponse))
	}))
	defer server.Close()

	client, err := GetAccessToken("user", "pass", server.URL, true)
	require.NoError(t, err)

	gql := NewGraphQLClient(client, server.URL, true)
	result, err := gql.UpdateAlertStatus(context.Background(), ObservabilityAlertStatusUpdateInput{
		AlertType: "METRIC",
		Input:     []HubPolicyIdInput{{PolicyID: "policy-123", NodeVersion: "MQ=="}},
		Status:    true,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, result.SuccessCount)
	assert.Equal(t, 0, result.FailureCount)
}

func TestUpdateAlertStatus_Disable(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == TokenEndpoint {
			mockAuthEndpoint(w)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(mockUpdateAlertStatusResponse))
	}))
	defer server.Close()

	client, err := GetAccessToken("user", "pass", server.URL, true)
	require.NoError(t, err)

	gql := NewGraphQLClient(client, server.URL, true)
	result, err := gql.UpdateAlertStatus(context.Background(), ObservabilityAlertStatusUpdateInput{
		AlertType: "METRIC",
		Input:     []HubPolicyIdInput{{PolicyID: "policy-123", NodeVersion: "MQ=="}},
		Status:    false,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, result.SuccessCount)
}

func TestUpdateAlertStatus_GraphQLError(t *testing.T) {
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
	result, err := gql.UpdateAlertStatus(context.Background(), ObservabilityAlertStatusUpdateInput{
		AlertType: "METRIC",
		Input:     []HubPolicyIdInput{{PolicyID: "policy-123", NodeVersion: "MQ=="}},
		Status:    true,
	})
	assert.Error(t, err)
	assert.Nil(t, result)
}
