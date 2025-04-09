package gorp

import (
	"resty.dev/v3"
)

const baseUrlFilters = "/api/v1/{project}/filter"

type filterClient struct {
	project string
	http    *resty.Client
}

// GetFiltersByName retrieves filter by its name
func (c *filterClient) GetFiltersByName(name string) (*FilterPage, error) {
	var filter FilterPage
	_, err := c.http.R().
		SetPathParam("project", c.project).
		SetQueryParam("filter.eq.name", name).
		SetResult(&filter).
		Get(baseUrlFilters)
	return &filter, err
}
