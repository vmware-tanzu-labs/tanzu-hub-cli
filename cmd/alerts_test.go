package cmd

import (
	"testing"

	"github.com/laidbackware/tanzu-hub-cli/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStripAlertIDs(t *testing.T) {
	t.Parallel()

	alerts := []api.ObservabilityMetricAlert{
		{
			PolicyID:          "id-1",
			NodeVersion:       "v1",
			PolicyName:        "Alert A",
			RunbookTemplateID: "rb-1",
			DashboardID:       "dash-1",
			EntityInfo:        api.AlertEntityInfo{EntityID: "eid-1", EntityName: "name-1"},
			AlertStatus:       "ACTIVE",
		},
	}

	result := stripAlertIDs(alerts)

	// Original is unchanged.
	assert.Equal(t, "id-1", alerts[0].PolicyID)

	// Stripped copy has IDs cleared but other fields preserved.
	r := result[0]
	assert.Empty(t, r.PolicyID)
	assert.Empty(t, r.NodeVersion)
	assert.Empty(t, r.RunbookTemplateID)
	assert.Empty(t, r.DashboardID)
	assert.Empty(t, r.EntityInfo.EntityID)
	assert.Equal(t, "Alert A", r.PolicyName)
	assert.Equal(t, "name-1", r.EntityInfo.EntityName)
	assert.Equal(t, "ACTIVE", r.AlertStatus)
}

func TestBuildAlertNameIndex(t *testing.T) {
	t.Parallel()

	alerts := []api.ObservabilityMetricAlert{
		{PolicyID: "id-1", PolicyName: "Alert A", NodeVersion: "v1"},
		{PolicyID: "id-2", PolicyName: "Alert B", NodeVersion: "v2"},
		{PolicyID: "id-3", PolicyName: "Alert C", NodeVersion: "v3"},
	}

	byName, duplicates := buildAlertNameIndex(alerts)
	assert.Len(t, byName, 3)
	assert.Empty(t, duplicates)
	assert.Equal(t, "id-1", byName["Alert A"].PolicyID)
	assert.Equal(t, "id-2", byName["Alert B"].PolicyID)
	assert.Equal(t, "id-3", byName["Alert C"].PolicyID)
}

func TestBuildAlertNameIndex_Duplicates(t *testing.T) {
	t.Parallel()

	alerts := []api.ObservabilityMetricAlert{
		{PolicyID: "id-1", PolicyName: "Alert A", NodeVersion: "v1"},
		{PolicyID: "id-2", PolicyName: "Alert A", NodeVersion: "v2"},
		{PolicyID: "id-3", PolicyName: "Alert B", NodeVersion: "v3"},
	}

	byName, duplicates := buildAlertNameIndex(alerts)
	assert.Len(t, byName, 2)
	assert.True(t, duplicates["Alert A"])
	assert.False(t, duplicates["Alert B"])
	// Last writer wins in the map.
	assert.Equal(t, "id-2", byName["Alert A"].PolicyID)
}

func TestBuildAlertNameIndex_Empty(t *testing.T) {
	t.Parallel()

	byName, duplicates := buildAlertNameIndex(nil)
	assert.Empty(t, byName)
	assert.Empty(t, duplicates)
}

func TestValidateAlertNames_Valid(t *testing.T) {
	t.Parallel()

	specs := []api.ObservabilityMetricAlert{
		{PolicyName: "Alert A"},
		{PolicyName: "Alert B"},
	}
	err := validateAlertNames(specs, map[string]bool{})
	assert.NoError(t, err)
}

func TestValidateAlertNames_WithIDsSkipped(t *testing.T) {
	t.Parallel()

	// Alerts with PolicyID set are not subject to name validation,
	// so duplicates among them are fine.
	specs := []api.ObservabilityMetricAlert{
		{PolicyID: "id-1", PolicyName: "Same Name"},
		{PolicyID: "id-2", PolicyName: "Same Name"},
	}
	err := validateAlertNames(specs, map[string]bool{})
	assert.NoError(t, err)
}

func TestValidateAlertNames_DuplicateOnServer(t *testing.T) {
	t.Parallel()

	specs := []api.ObservabilityMetricAlert{
		{PolicyName: "Dup Alert"},
	}
	duplicates := map[string]bool{"Dup Alert": true}
	err := validateAlertNames(specs, duplicates)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "multiple existing alerts share the name")
	assert.Contains(t, err.Error(), "Dup Alert")
}

func TestValidateAlertNames_DuplicateInInput(t *testing.T) {
	t.Parallel()

	specs := []api.ObservabilityMetricAlert{
		{PolicyName: "My Alert"},
		{PolicyName: "My Alert"},
	}
	err := validateAlertNames(specs, map[string]bool{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "input contains duplicate alert name")
	assert.Contains(t, err.Error(), "My Alert")
}

func TestValidateAlertNames_DuplicateInInputWithIDBypass(t *testing.T) {
	t.Parallel()

	// First has an ID so it's skipped; second doesn't, so it's the only
	// entry in inputNames — no duplicate error.
	specs := []api.ObservabilityMetricAlert{
		{PolicyID: "id-1", PolicyName: "My Alert"},
		{PolicyName: "My Alert"},
	}
	err := validateAlertNames(specs, map[string]bool{})
	assert.NoError(t, err)
}
