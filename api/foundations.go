package api

import (
	"context"
	"fmt"

	graphql "github.com/hasura/go-graphql-client"
)

type ManagementEndpointSummary struct {
	ManagementEndpointID string `graphql:"managementEndpointId"`
	EndpointName         string `graphql:"endpointName"`
	Environment          string `graphql:"environment"`
}

type ManagementEndpointCollectorDeploymentProperty struct {
	Name  string      `graphql:"name"`
	Value interface{} `graphql:"value"`
}

type ManagementEndpointCollectorCredentialsProperties struct {
	ClientID     string `graphql:"clientId"`
	ClientSecret string `graphql:"clientSecret"`
}

type ManagementEndpointCollectorCredentials struct {
	Properties *ManagementEndpointCollectorCredentialsProperties `graphql:"properties"`
}

type AttachCollectorResult struct {
	Name                                 string                    `graphql:"name"`
	ID                                     string                                   `graphql:"id"`
	DeploymentInstall                      string                                   `graphql:"deploymentInstall"`
	DeploymentProperties                   []ManagementEndpointCollectorDeploymentProperty `graphql:"deploymentProperties"`
	ManagementEndpoint                     ManagementEndpointSummary                `graphql:"managementEndpoint"`
	ManagementEndpointCollectorCredentials ManagementEndpointCollectorCredentials   `graphql:"managementEndpointCollectorCredentials"`
}

type ManagementEndpointCollector struct {
	Name                                 string                    `graphql:"name"`
	Type                                 string                    `graphql:"type"`
	ManagementEndpointCollectorTypeVersion string                  `graphql:"managementEndpointCollectorTypeVersion"`
	LatestAvailableVersion               string                    `graphql:"latestAvailableVersion"`
	HealthStatus                         string                    `graphql:"healthStatus"`
	Status                               string                    `graphql:"status"`
	LastUpdateTime                       string                    `graphql:"lastUpdateTime"`
	ID                                   string                    `graphql:"id"`
	ManagementEndpoint                   ManagementEndpointSummary `graphql:"managementEndpoint"`
}

var attachmentReturnFields = []string{
	"cluster-cloud-account-id",
	"name",
	"oauth-app-id",
	"oauth-app-secret",
	"foundationId",
	"collector-id",
	"org-id",
	"lemans-gateway-base-url",
	"caBundle",
}

type ManagementEndpointSelfManagedCollectorInput struct {
	CollectorName string `json:"collectorName"`
	CollectorType string `json:"collectorType,omitempty"`
}

func (g *GraphQLClient) mutateSelfManagedCollector(ctx context.Context, collectorName, collectorType string) (map[string]string, error) {
	var mutation struct {
		ManagementEndpointMutation struct {
			AttachSelfManagedManagementEndpointCollector AttachCollectorResult `graphql:"attachSelfManagedManagementEndpointCollector(input: $input)"`
		} `graphql:"managementEndpointMutation"`
	}

	vars := map[string]any{
		"input": ManagementEndpointSelfManagedCollectorInput{
			CollectorName: collectorName,
			CollectorType: collectorType,
		},
	}

	if err := g.Client.Mutate(ctx, &mutation, vars); err != nil {
		return nil, err
	}

	r := mutation.ManagementEndpointMutation.AttachSelfManagedManagementEndpointCollector

	result := map[string]string{
		"id":             r.ID,
		"name":           r.Name,
		"foundationName": r.ManagementEndpoint.EndpointName,
	}

	allowedFields := make(map[string]struct{}, len(attachmentReturnFields))
	for _, f := range attachmentReturnFields {
		allowedFields[f] = struct{}{}
	}

	for _, prop := range r.DeploymentProperties {
		if _, allowed := allowedFields[prop.Name]; !allowed {
			continue
		}
		if s, ok := prop.Value.(string); ok {
			// rename to make human readable
			if prop.Name == "lemans-gateway-base-url" {
				result["ingestion_url"] = s
				continue
			}
			if prop.Name == "cluster-cloud-account-id" {
				result["foundation-id"] = s
				continue
			}
			result[prop.Name] = s
		}
	}

	return result, nil
}

func (g *GraphQLClient) AttachSelfManagedCollector(ctx context.Context, collectorName, collectorType, managementEndpointID string) (map[string]string, error) {
	var existsQuery struct {
		ManagementEndpointQuery struct {
			QueryManagementEndpointCollectors struct {
				ManagementEndpointCollectors []struct {
					ID graphql.ID `graphql:"id"`
				} `graphql:"managementEndpointCollectors"`
			} `graphql:"queryManagementEndpointCollectors(name: $name)"`
		} `graphql:"managementEndpointQuery"`
	}

	if err := g.Client.Query(ctx, &existsQuery, map[string]any{"name": []string{collectorName}}); err != nil {
		return nil, fmt.Errorf("checking for existing collector: %w", err)
	}
	if len(existsQuery.ManagementEndpointQuery.QueryManagementEndpointCollectors.ManagementEndpointCollectors) > 0 {
		return nil, fmt.Errorf("collector with name %q already exists", collectorName)
	}

	return g.mutateSelfManagedCollector(ctx, collectorName, collectorType)
}

func (g *GraphQLClient) UpdateSelfManagedCollector(ctx context.Context, collectorName, collectorType, managementEndpointID string) (map[string]string, error) {
	return g.mutateSelfManagedCollector(ctx, collectorName, collectorType)
}

func (g *GraphQLClient) DeleteFoundationByName(ctx context.Context, name string) error {
	var lookupQuery struct {
		ManagementEndpointQuery struct {
			QueryManagementEndpointCollectors struct {
				ManagementEndpointCollectors []struct {
					ID graphql.ID `graphql:"id"`
				} `graphql:"managementEndpointCollectors"`
			} `graphql:"queryManagementEndpointCollectors(name: $name)"`
		} `graphql:"managementEndpointQuery"`
	}

	if err := g.Client.Query(ctx, &lookupQuery, map[string]any{"name": []string{name}}); err != nil {
		return err
	}

	endpoints := lookupQuery.ManagementEndpointQuery.QueryManagementEndpointCollectors.ManagementEndpointCollectors
	if len(endpoints) == 0 {
		return fmt.Errorf("no foundation found with name %q", name)
	}

	var deleteMutation struct {
		ManagementEndpointMutation struct {
			DetachManagementEndpointCollector struct {
				ID graphql.ID `graphql:"id"`
			} `graphql:"detachManagementEndpointCollector(id: $id)"`
		} `graphql:"managementEndpointMutation"`
	}

	if err := g.Client.Mutate(ctx, &deleteMutation, map[string]any{"id": endpoints[0].ID}); err != nil {
		return err
	}

	return nil
}

func (g *GraphQLClient) QueryManagementEndpoints(ctx context.Context) ([]ManagementEndpointCollector, error) {
	var query struct {
		ManagementEndpointQuery struct {
			QueryManagementEndpointCollectors struct {
				ManagementEndpointCollectors []ManagementEndpointCollector `graphql:"managementEndpointCollectors"`
			} `graphql:"queryManagementEndpointCollectors"`
		} `graphql:"managementEndpointQuery"`
	}

	if err := g.Client.Query(ctx, &query, nil); err != nil {
		return nil, err
	}

	return query.ManagementEndpointQuery.QueryManagementEndpointCollectors.ManagementEndpointCollectors, nil
}

