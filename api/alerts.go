package api

import (
	"context"
	"regexp"
	"strings"
)

var htmlTagRe = regexp.MustCompile(`<[^>]+>`)

func stripHTML(s string) string {
	return strings.TrimSpace(htmlTagRe.ReplaceAllString(s, ""))
}

// Input types — json tags for hasura variable serialization.

type QuerySort struct {
	Field string `json:"field"`
	Order string `json:"order"`
}

type ObservabilityAlertFilterInput struct {
	AlertStatus       string   `json:"alertStatus,omitempty"`
	AlertType         string   `json:"alertType,omitempty"`
	CreatedBy         string   `json:"createdBy,omitempty"`
	Enabled           *bool    `json:"enabled,omitempty"`
	EntityID          []string `json:"entityId,omitempty"`
	EntityType        []string `json:"entityType,omitempty"`
	Name              []string `json:"name,omitempty"`
	PolicyID          []string `json:"policyId,omitempty"`
	RunbookTemplateID string   `json:"runbookTemplateId,omitempty"`
	Source            string   `json:"source,omitempty"`
	ThresholdType     string   `json:"thresholdType,omitempty"`
	UpdatedBy         string   `json:"updatedBy,omitempty"`
}

// Response types — graphql tags for hasura client deserialization.

type AlertPageInfo struct {
	HasPreviousPage bool   `graphql:"hasPreviousPage" json:"hasPreviousPage" yaml:"hasPreviousPage"`
	HasNextPage     bool   `graphql:"hasNextPage"     json:"hasNextPage"     yaml:"hasNextPage"`
	StartCursor     string `graphql:"startCursor"     json:"startCursor"     yaml:"startCursor"`
	EndCursor       string `graphql:"endCursor"       json:"endCursor"       yaml:"endCursor"`
}

type AlertUser struct {
	UserAccount string `graphql:"userAccount" json:"userAccount" yaml:"userAccount"`
}

type AlertEntityInfo struct {
	EntityID   string `graphql:"entityId"   json:"entityId"   yaml:"entityId"`
	EntityName string `graphql:"entityName" json:"entityName" yaml:"entityName"`
	EntityType string `graphql:"entityType" json:"entityType" yaml:"entityType"`
}

type AlertDataScopedEntities struct {
	EntityIDs  []string `graphql:"entityIds"  json:"entityIds"  yaml:"entityIds"`
	EntityType string   `graphql:"entityType" json:"entityType" yaml:"entityType"`
}

type AlertTag struct {
	Key   string `graphql:"key"   json:"key"   yaml:"key"`
	Value string `graphql:"value" json:"value" yaml:"value"`
}

type AlertThresholdCondition struct {
	AlertSeverity  string  `graphql:"alertSeverity"  json:"alertSeverity"  yaml:"alertSeverity"`
	ThresholdValue float64 `graphql:"thresholdValue" json:"thresholdValue" yaml:"thresholdValue"`
}

type AlertDuration struct {
	Duration     int    `graphql:"duration"     json:"duration"     yaml:"duration"`
	DurationUnit string `graphql:"durationUnit" json:"durationUnit" yaml:"durationUnit"`
}

type AlertOverrideRule struct {
	LabelName   string                  `graphql:"labelName"   json:"labelName"   yaml:"labelName"`
	LabelValues []string                `graphql:"labelValues" json:"labelValues" yaml:"labelValues"`
	RuleName    string                  `graphql:"ruleName"    json:"ruleName"    yaml:"ruleName"`
	Tags        []AlertTag              `graphql:"tags"        json:"tags"        yaml:"tags"`
	Threshold   AlertThresholdCondition `graphql:"threshold"   json:"threshold"   yaml:"threshold"`
}

type MetricAlertCondition struct {
	IncludedEntityIDs                []string                `graphql:"includedEntityIds"                json:"includedEntityIds"                yaml:"includedEntityIds"`
	Threshold                        AlertThresholdCondition `graphql:"threshold"                        json:"threshold"                        yaml:"threshold"`
	Operator                         string                  `graphql:"operator"                         json:"operator"                         yaml:"operator"`
	Tags                             []AlertTag              `graphql:"tags"                             json:"tags"                             yaml:"tags"`
	OverrideRules                    []AlertOverrideRule     `graphql:"overrideRules"                    json:"overrideRules"                    yaml:"overrideRules"`
	TriggerWindowDuration            AlertDuration           `graphql:"triggerWindowDuration"            json:"triggerWindowDuration"            yaml:"triggerWindowDuration"`
	ResolveWindowDuration            AlertDuration           `graphql:"resolveWindowDuration"            json:"resolveWindowDuration"            yaml:"resolveWindowDuration"`
	AlertEvaluationFrequencyDuration AlertDuration           `graphql:"alertEvaluationFrequencyDuration" json:"alertEvaluationFrequencyDuration" yaml:"alertEvaluationFrequencyDuration"`
}

type MetricAlertRule struct {
	QueryString         string               `graphql:"queryString"         json:"queryString"         yaml:"queryString"`
	QueryStringMetadata string               `graphql:"queryStringMetadata" json:"queryStringMetadata" yaml:"queryStringMetadata"`
	Namespace           string               `graphql:"namespace"           json:"namespace"           yaml:"namespace"`
	AlertCondition      MetricAlertCondition `graphql:"alertCondition"      json:"alertCondition"      yaml:"alertCondition"`
}

type ObservabilityMetricAlert struct {
	NodeVersion        string                  `graphql:"nodeVersion"        json:"nodeVersion"        yaml:"nodeVersion"`
	PolicyID           string                  `graphql:"policyId"           json:"policyId"           yaml:"policyId"`
	PolicyName         string                  `graphql:"policyName"         json:"policyName"         yaml:"policyName"`
	Description        string                  `graphql:"description"        json:"description"        yaml:"description"`
	EntityType         string                  `graphql:"entityType"         json:"entityType"         yaml:"entityType"`
	Enabled            bool                    `graphql:"enabled"            json:"enabled"            yaml:"enabled"`
	RunbookTemplateID  string                  `graphql:"runbookTemplateId"  json:"runbookTemplateId"  yaml:"runbookTemplateId"`
	DashboardID        string                  `graphql:"dashboardId"        json:"dashboardId"        yaml:"dashboardId"`
	DataScopedEntities AlertDataScopedEntities `graphql:"dataScopedEntities" json:"dataScopedEntities" yaml:"dataScopedEntities"`
	EntityInfo         AlertEntityInfo         `graphql:"entityInfo"         json:"entityInfo"         yaml:"entityInfo"`
	CreatedBy          AlertUser               `graphql:"createdBy"          json:"createdBy"          yaml:"createdBy"`
	CreationTime       string                  `graphql:"creationTime"       json:"creationTime"       yaml:"creationTime"`
	UpdatedBy          AlertUser               `graphql:"updatedBy"          json:"updatedBy"          yaml:"updatedBy"`
	LastUpdateTime     string                  `graphql:"lastUpdateTime"     json:"lastUpdateTime"     yaml:"lastUpdateTime"`
	LastRunTime        string                  `graphql:"lastRunTime"        json:"lastRunTime"        yaml:"lastRunTime"`
	PolicyProvider     string                  `graphql:"policyProvider"     json:"policyProvider"     yaml:"policyProvider"`
	Source             string                  `graphql:"source"             json:"source"             yaml:"source"`
	AlertType          string                  `graphql:"alertType"          json:"alertType"          yaml:"alertType"`
	AlertStatus        string                  `graphql:"alertStatus"        json:"alertStatus"        yaml:"alertStatus"`
	IsCustomAlert      bool                    `graphql:"isCustomAlert"      json:"isCustomAlert"      yaml:"isCustomAlert"`
	ThresholdType      string                  `graphql:"thresholdType"      json:"thresholdType"      yaml:"thresholdType"`
	Rule               MetricAlertRule         `graphql:"rule"               json:"rule"               yaml:"rule"`
}

type ObservabilityAlertQueryResult struct {
	Alerts     []ObservabilityMetricAlert
	PageInfo   AlertPageInfo
	Count      int
	TotalCount int
}

type alertPoliciesResult struct {
	Count      int           `graphql:"count"`
	TotalCount int           `graphql:"totalCount"`
	PageInfo   AlertPageInfo `graphql:"pageInfo"`
	Policies   []struct {
		Typename    string                   `graphql:"__typename"`
		MetricAlert ObservabilityMetricAlert `graphql:"... on ObservabilityMetricAlert"`
	} `graphql:"policies"`
}

func (g *GraphQLClient) QueryObservabilityAlerts(ctx context.Context, filter ObservabilityAlertFilterInput, sort []QuerySort, first int, after, before string) (*ObservabilityAlertQueryResult, error) {
	vars := map[string]any{
		"SORT":   sort,
		"FIRST":  first,
		"FILTER": filter,
	}

	var qhp alertPoliciesResult
	if after != "" {
		var query struct {
			HubPolicyQuery struct {
				Providers struct {
					ObservabilityAlert struct {
						QueryHubPolicies alertPoliciesResult `graphql:"queryHubPolicies(sort: $SORT, first: $FIRST, after: $AFTER, alertFilter: $FILTER)"`
					} `graphql:"observabilityAlert"`
				} `graphql:"providers"`
			} `graphql:"hubPolicyQuery"`
		}
		vars["AFTER"] = after
		if err := g.Client.Query(ctx, &query, vars); err != nil {
			return nil, err
		}
		qhp = query.HubPolicyQuery.Providers.ObservabilityAlert.QueryHubPolicies
	} else if before != "" {
		var query struct {
			HubPolicyQuery struct {
				Providers struct {
					ObservabilityAlert struct {
						QueryHubPolicies alertPoliciesResult `graphql:"queryHubPolicies(sort: $SORT, first: $FIRST, before: $BEFORE, alertFilter: $FILTER)"`
					} `graphql:"observabilityAlert"`
				} `graphql:"providers"`
			} `graphql:"hubPolicyQuery"`
		}
		vars["BEFORE"] = before
		if err := g.Client.Query(ctx, &query, vars); err != nil {
			return nil, err
		}
		qhp = query.HubPolicyQuery.Providers.ObservabilityAlert.QueryHubPolicies
	} else {
		var query struct {
			HubPolicyQuery struct {
				Providers struct {
					ObservabilityAlert struct {
						QueryHubPolicies alertPoliciesResult `graphql:"queryHubPolicies(sort: $SORT, first: $FIRST, alertFilter: $FILTER)"`
					} `graphql:"observabilityAlert"`
				} `graphql:"providers"`
			} `graphql:"hubPolicyQuery"`
		}
		if err := g.Client.Query(ctx, &query, vars); err != nil {
			return nil, err
		}
		qhp = query.HubPolicyQuery.Providers.ObservabilityAlert.QueryHubPolicies
	}

	alerts := make([]ObservabilityMetricAlert, 0, len(qhp.Policies))
	for _, p := range qhp.Policies {
		alerts = append(alerts, p.MetricAlert)
	}

	return &ObservabilityAlertQueryResult{
		Alerts:     alerts,
		PageInfo:   qhp.PageInfo,
		Count:      qhp.Count,
		TotalCount: qhp.TotalCount,
	}, nil
}

// Mutation input types

type MetricAlertTagInput struct {
	Key   string `json:"key"`
	Value string `json:"value,omitempty"`
}

type MetricAlertThresholdInput struct {
	AlertSeverity  string  `json:"alertSeverity"`
	ThresholdValue float64 `json:"thresholdValue"`
}

type MetricAlertOverrideRuleInput struct {
	LabelName   string                    `json:"labelName"`
	LabelValues []string                  `json:"labelValues,omitempty"`
	RuleName    string                    `json:"ruleName"`
	Tags        []MetricAlertTagInput     `json:"tags,omitempty"`
	Threshold   MetricAlertThresholdInput `json:"threshold"`
}

type MetricAlertConditionInput struct {
	AlertEvaluationFrequencyInMins int                            `json:"alertEvaluationFrequencyInMins"`
	IncludedEntityIDs              []string                       `json:"includedEntityIds,omitempty"`
	Operator                       string                         `json:"operator"`
	OverrideRules                  []MetricAlertOverrideRuleInput `json:"overrideRules,omitempty"`
	ResolveWindowInMins            int                            `json:"resolveWindowInMins"`
	Tags                           []MetricAlertTagInput          `json:"tags,omitempty"`
	Threshold                      MetricAlertThresholdInput      `json:"threshold"`
	TriggerWindowInMins            int                            `json:"triggerWindowInMins"`
}

type MetricAlertRuleInput struct {
	AlertConditionInput MetricAlertConditionInput `json:"alertConditionInput"`
	Namespace           string                    `json:"namespace,omitempty"`
	QueryString         string                    `json:"queryString"`
	QueryStringMetadata string                    `json:"queryStringMetadata,omitempty"`
	ThresholdType       string                    `json:"thresholdType"`
}

type ObservabilityMetricAlertCreateInput struct {
	Description       string               `json:"description,omitempty"`
	EntityID          string               `json:"entityId,omitempty"`
	EntityType        string               `json:"entityType"`
	Name              string               `json:"name"`
	Rule              MetricAlertRuleInput `json:"rule"`
	RunbookTemplateID string               `json:"runbookTemplateId,omitempty"`
	Source            string               `json:"source"`
}

type ObservabilityMetricAlertUpdateInput struct {
	Description       string               `json:"description,omitempty"`
	EntityID          string               `json:"entityId,omitempty"`
	EntityType        string               `json:"entityType"`
	Name              string               `json:"name"`
	NodeVersion       string               `json:"nodeVersion"`
	PolicyID          string               `json:"policyId"`
	Rule              MetricAlertRuleInput `json:"rule"`
	RunbookTemplateID string               `json:"runbookTemplateId,omitempty"`
	Source            string               `json:"source"`
}

// toMinutes converts an AlertDuration to whole minutes.
func toMinutes(d AlertDuration) int {
	switch strings.ToUpper(d.DurationUnit) {
	case "HOURS":
		return d.Duration * 60
	case "SECONDS":
		if d.Duration < 60 {
			return 1
		}
		return d.Duration / 60
	default: // MINUTES or empty
		return d.Duration
	}
}

func buildAlertRuleInput(a ObservabilityMetricAlert) MetricAlertRuleInput {
	cond := a.Rule.AlertCondition

	tags := make([]MetricAlertTagInput, len(cond.Tags))
	for i, t := range cond.Tags {
		tags[i] = MetricAlertTagInput(t)
	}

	overrideRules := make([]MetricAlertOverrideRuleInput, len(cond.OverrideRules))
	for i, r := range cond.OverrideRules {
		orTags := make([]MetricAlertTagInput, len(r.Tags))
		for j, t := range r.Tags {
			orTags[j] = MetricAlertTagInput(t)
		}
		overrideRules[i] = MetricAlertOverrideRuleInput{
			LabelName:   r.LabelName,
			LabelValues: r.LabelValues,
			RuleName:    r.RuleName,
			Tags:        orTags,
			Threshold: MetricAlertThresholdInput{
				AlertSeverity:  r.Threshold.AlertSeverity,
				ThresholdValue: r.Threshold.ThresholdValue,
			},
		}
	}

	thresholdType := a.ThresholdType
	if thresholdType == "" {
		thresholdType = "STATIC"
	}

	return MetricAlertRuleInput{
		QueryString:         a.Rule.QueryString,
		QueryStringMetadata: a.Rule.QueryStringMetadata,
		Namespace:           a.Rule.Namespace,
		ThresholdType:       thresholdType,
		AlertConditionInput: MetricAlertConditionInput{
			AlertEvaluationFrequencyInMins: toMinutes(cond.AlertEvaluationFrequencyDuration),
			IncludedEntityIDs:              cond.IncludedEntityIDs,
			Operator:                       cond.Operator,
			OverrideRules:                  overrideRules,
			ResolveWindowInMins:            toMinutes(cond.ResolveWindowDuration),
			Tags:                           tags,
			Threshold: MetricAlertThresholdInput{
				AlertSeverity:  cond.Threshold.AlertSeverity,
				ThresholdValue: cond.Threshold.ThresholdValue,
			},
			TriggerWindowInMins: toMinutes(cond.TriggerWindowDuration),
		},
	}
}

func AlertToCreateInput(a ObservabilityMetricAlert) ObservabilityMetricAlertCreateInput {
	return ObservabilityMetricAlertCreateInput{
		Description:       stripHTML(a.Description),
		EntityID:          a.EntityInfo.EntityID,
		EntityType:        a.EntityType,
		Name:              a.PolicyName,
		Rule:              buildAlertRuleInput(a),
		RunbookTemplateID: a.RunbookTemplateID,
		Source:            a.Source,
	}
}

func AlertToUpdateInput(a ObservabilityMetricAlert) ObservabilityMetricAlertUpdateInput {
	return ObservabilityMetricAlertUpdateInput{
		Description:       stripHTML(a.Description),
		EntityID:          a.EntityInfo.EntityID,
		EntityType:        a.EntityType,
		Name:              a.PolicyName,
		NodeVersion:       a.NodeVersion,
		PolicyID:          a.PolicyID,
		Rule:              buildAlertRuleInput(a),
		RunbookTemplateID: a.RunbookTemplateID,
		Source:            a.Source,
	}
}

// alertMutationResult is a minimal return type for alert mutations.
// ObservabilityMetricAlert contains fields (e.g. dataScopedEntities) that are
// not present on the mutation return type in the schema, so we only query what
// we need from the response.
type alertMutationResult struct {
	PolicyID   string `graphql:"policyId"`
	PolicyName string `graphql:"policyName"`
}

func (g *GraphQLClient) CreateMetricAlert(ctx context.Context, input ObservabilityMetricAlertCreateInput) (*alertMutationResult, error) {
	var mutation struct {
		HubPolicyMutation struct {
			Providers struct {
				ObservabilityAlert struct {
					CreateMetricAlert alertMutationResult `graphql:"createMetricAlert(input: $input)"`
				} `graphql:"observabilityAlert"`
			} `graphql:"providers"`
		} `graphql:"hubPolicyMutation"`
	}

	if err := g.Client.Mutate(ctx, &mutation, map[string]any{"input": input}); err != nil {
		return nil, err
	}

	result := mutation.HubPolicyMutation.Providers.ObservabilityAlert.CreateMetricAlert
	return &result, nil
}

type HubPolicyIdInput struct {
	NodeVersion string `json:"nodeVersion"`
	PolicyID    string `json:"policyId"`
}

type alertDeleteResult struct {
	FailureCount int `graphql:"failureCount"`
	SuccessCount int `graphql:"successCount"`
}

func (g *GraphQLClient) DeleteAlerts(ctx context.Context, input []HubPolicyIdInput) (*alertDeleteResult, error) {
	var mutation struct {
		HubPolicyMutation struct {
			Providers struct {
				ObservabilityAlert struct {
					DeleteAlerts alertDeleteResult `graphql:"deleteAlerts(input: $input)"`
				} `graphql:"observabilityAlert"`
			} `graphql:"providers"`
		} `graphql:"hubPolicyMutation"`
	}

	if err := g.Client.Mutate(ctx, &mutation, map[string]any{"input": input}); err != nil {
		return nil, err
	}

	result := mutation.HubPolicyMutation.Providers.ObservabilityAlert.DeleteAlerts
	return &result, nil
}

type ObservabilityAlertStatusUpdateInput struct {
	AlertType string             `json:"alertType"`
	Input     []HubPolicyIdInput `json:"input"`
	Status    bool               `json:"status"`
}

func (g *GraphQLClient) UpdateAlertStatus(ctx context.Context, input ObservabilityAlertStatusUpdateInput) (*alertDeleteResult, error) {
	var mutation struct {
		HubPolicyMutation struct {
			Providers struct {
				ObservabilityAlert struct {
					UpdateAlertStatus alertDeleteResult `graphql:"updateAlertStatus(input: $input)"`
				} `graphql:"observabilityAlert"`
			} `graphql:"providers"`
		} `graphql:"hubPolicyMutation"`
	}

	if err := g.Client.Mutate(ctx, &mutation, map[string]any{"input": input}); err != nil {
		return nil, err
	}

	result := mutation.HubPolicyMutation.Providers.ObservabilityAlert.UpdateAlertStatus
	return &result, nil
}

func (g *GraphQLClient) UpdateMetricAlert(ctx context.Context, input ObservabilityMetricAlertUpdateInput) (*alertMutationResult, error) {
	var mutation struct {
		HubPolicyMutation struct {
			Providers struct {
				ObservabilityAlert struct {
					UpdateMetricAlert alertMutationResult `graphql:"updateMetricAlert(input: $input)"`
				} `graphql:"observabilityAlert"`
			} `graphql:"providers"`
		} `graphql:"hubPolicyMutation"`
	}

	if err := g.Client.Mutate(ctx, &mutation, map[string]any{"input": input}); err != nil {
		return nil, err
	}

	result := mutation.HubPolicyMutation.Providers.ObservabilityAlert.UpdateMetricAlert
	return &result, nil
}
