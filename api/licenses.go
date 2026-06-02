package api

import "context"

type LicenseAttribute struct {
	Name  string
	Value string
}

type License struct {
	ID                 string             `graphql:"id"`
	Key                string             `graphql:"key"`
	LicenseVersion     string             `graphql:"licenseVersion"`
	ProductID          string             `graphql:"productId"`
	ProductDescription string             `graphql:"productDescription"`
	FoundationCount    int                `graphql:"foundationCount"`
	Expiration         string             `graphql:"expiration"`
	ExtendedAttributes []LicenseAttribute `graphql:"extendedAttributes"`
}

func (g *GraphQLClient) QueryLicenses(ctx context.Context) ([]License, error) {
	var query struct {
		LicenseQuery struct {
			QueryLicenses []License `graphql:"queryLicenses"`
		} `graphql:"licenseQuery"`
	}

	if err := g.Client.Query(ctx, &query, nil); err != nil {
		return nil, err
	}

	return query.LicenseQuery.QueryLicenses, nil
}

type LicenseAddResult struct {
	Success            bool    `graphql:"success"`
	VerificationFailure string `graphql:"verificationFailure"`
	License            *License `graphql:"license"`
}

func (g *GraphQLClient) AddLicense(ctx context.Context, licenseKey string) (*LicenseAddResult, error) {
	var mutation struct {
		LicenseMutation struct {
			AddLicense LicenseAddResult `graphql:"addLicense(licenseKey: $licenseKey)"`
		} `graphql:"licenseMutation"`
	}

	vars := map[string]any{
		"licenseKey": licenseKey,
	}

	if err := g.Client.Mutate(ctx, &mutation, vars); err != nil {
		return nil, err
	}

	result := mutation.LicenseMutation.AddLicense
	return &result, nil
}

func (g *GraphQLClient) DeleteLicense(ctx context.Context, licenseID string) (bool, error) {
	var mutation struct {
		LicenseMutation struct {
			DeleteLicense struct {
				Success bool `graphql:"success"`
			} `graphql:"deleteLicense(licenseId: $licenseId)"`
		} `graphql:"licenseMutation"`
	}

	vars := map[string]any{
		"licenseId": licenseID,
	}

	if err := g.Client.Mutate(ctx, &mutation, vars); err != nil {
		return false, err
	}

	return mutation.LicenseMutation.DeleteLicense.Success, nil
}

type LicenseApplicationFoundationWorkflow struct {
	FoundationID string   `graphql:"foundationId"`
	Status       string   `graphql:"status"`
	WorkflowID   string   `graphql:"workflowId"`
	Errors       []string `graphql:"errors"`
}

type LicenseApplyResult struct {
	LicenseApplicationResults []LicenseApplicationFoundationWorkflow `graphql:"licenseApplicationResults"`
}

func (g *GraphQLClient) ApplyLicenseToFoundations(ctx context.Context, licenseID string, foundationIDs []string) (*LicenseApplyResult, error) {
	var mutation struct {
		LicenseMutation struct {
			ApplyLicenseToFoundations LicenseApplyResult `graphql:"applyLicenseToFoundations(input: $input)"`
		} `graphql:"licenseMutation"`
	}

	type LicenseApplyToFoundationsInput struct {
		LicenseID     string   `json:"licenseId"`
		FoundationIDs []string `json:"foundationIds"`
	}

	vars := map[string]any{
		"input": LicenseApplyToFoundationsInput{
			LicenseID:     licenseID,
			FoundationIDs: foundationIDs,
		},
	}

	if err := g.Client.Mutate(ctx, &mutation, vars); err != nil {
		return nil, err
	}

	result := mutation.LicenseMutation.ApplyLicenseToFoundations
	return &result, nil
}