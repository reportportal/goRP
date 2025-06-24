package gorp

import (
	"context"

	"resty.dev/v3"

	"github.com/reportportal/goRP/pkg/openapi"
)

const baseUrlFilters = "/api/v1/{project}/filter"

type filterClient struct {
	http *resty.Client
}

// GetFiltersByName retrieves filter by its name
func (c *filterClient) GetFiltersByName(
	ctx context.Context,
	project, name string,
) (*openapi.PageUserFilterResource, error) {
	var filter openapi.PageUserFilterResource
	_, err := c.http.R().
		SetContext(ctx).
		SetPathParam("project", project).
		SetQueryParam("filter.eq.name", name).
		SetResult(&filter).
		Get(baseUrlFilters)
	return &filter, err
}
