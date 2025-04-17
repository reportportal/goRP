package gorp

import (
	"resty.dev/v3"

	"github.com/reportportal/goRP/v5/pkg/openapi"
)

const baseUrlFilters = "/api/v1/{project}/filter"

type filterClient struct {
	project string
	http    *resty.Client
}

// GetFiltersByName retrieves filter by its name
func (c *filterClient) GetFiltersByName(name string) (*openapi.PageUserFilterResource, error) {
	var filter openapi.PageUserFilterResource
	_, err := c.http.R().
		SetPathParam("project", c.project).
		SetQueryParam("filter.eq.name", name).
		SetResult(&filter).
		Get(baseUrlFilters)
	return &filter, err
}
