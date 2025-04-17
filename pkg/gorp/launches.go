package gorp

import (
	"context"
	"fmt"

	"resty.dev/v3"

	"github.com/reportportal/goRP/v5/pkg/openapi"
)

const baseUrlLaunch = "/api/v1/{project}/launch"

type launchClient struct {
	http *resty.Client
}

// GetLaunchesByFilterString retrieves launches by filter as string
func (c *launchClient) GetLaunchesByFilterString(ctx context.Context, project, filter string) (*openapi.PageLaunchResource, error) {
	var launches openapi.PageLaunchResource
	_, err := c.http.R().SetContext(ctx).
		SetPathParam("project", project).
		SetResult(&launches).
		SetQueryString(filter).
		Get(baseUrlLaunch)
	return &launches, err
}

// GetLaunchesByFilterName retrieves launches by filter name
func (c *launchClient) GetLaunchesByFilterName(ctx context.Context, project, name string) (*openapi.PageLaunchResource, error) {
	filter, err := (&filterClient{http: c.http}).GetFiltersByName(ctx, project, name)
	if err != nil {
		return nil, err
	}

	if filter.Page.GetSize() < 1 || len(filter.Content) == 0 {
		return nil, fmt.Errorf("no filter %s found", name) //nolint:err113 //dynamic error is intentional
	}

	var launches openapi.PageLaunchResource
	params := ConvertToFilterParams(filter.Content[0])
	_, err = c.http.R().
		SetContext(ctx).
		SetPathParam("project", project).
		SetResult(&launches).
		SetQueryParamsFromValues(params).
		Get(baseUrlLaunch)
	return &launches, err
}
