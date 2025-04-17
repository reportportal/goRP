package gorp

import (
	"fmt"

	"resty.dev/v3"

	"github.com/reportportal/goRP/v5/pkg/openapi"
)

type launchClient struct {
	project string
	http    *resty.Client
}

// GetLaunches retrieves latest launches
func (c *launchClient) GetLaunches() (*openapi.PageLaunchResource, error) {
	var launches openapi.PageLaunchResource
	_, err := c.http.R().
		SetPathParam("project", c.project).
		SetResult(&launches).
		Get(baseUrlLaunch)
	return &launches, err
}

// GetLaunchesPage retrieves latest launches with paging
func (c *launchClient) GetLaunchesPage(paging PageDetails) (*openapi.PageLaunchResource, error) {
	var launches openapi.PageLaunchResource
	_, err := c.http.R().
		SetPathParam("project", c.project).
		SetResult(&launches).
		Funcs(addPaging(paging)).
		Get(baseUrlLaunch)
	return &launches, err
}

// GetLaunchesByFilter retrieves launches by filter
func (c *launchClient) GetLaunchesByFilter(filter map[string]string) (*openapi.PageLaunchResource, error) {
	var launches openapi.PageLaunchResource
	_, err := c.http.R().
		SetPathParam("project", c.project).
		SetResult(&launches).
		SetQueryParams(filter).
		Get(baseUrlLaunch)
	return &launches, err
}

// GetLaunchesByFilterPage retrieves launches by filter with paging
func (c *launchClient) GetLaunchesByFilterPage(filter map[string]string, paging PageDetails) (*openapi.PageLaunchResource, error) {
	var launches openapi.PageLaunchResource
	_, err := c.http.R().
		SetPathParam("project", c.project).
		SetResult(&launches).
		SetQueryParams(filter).
		Funcs(addPaging(paging)).
		Get(baseUrlLaunch)
	return &launches, err
}

// GetLaunchesByFilterString retrieves launches by filter as string
func (c *launchClient) GetLaunchesByFilterString(filter string) (*openapi.PageLaunchResource, error) {
	var launches openapi.PageLaunchResource
	_, err := c.http.R().
		SetPathParam("project", c.project).
		SetResult(&launches).
		SetQueryString(filter).
		Get(baseUrlLaunch)
	return &launches, err
}

// GetLaunchesByFilterName retrieves launches by filter name
func (c *launchClient) GetLaunchesByFilterName(name string) (*openapi.PageLaunchResource, error) {
	filter, err := (&filterClient{project: c.project, http: c.http}).GetFiltersByName(name)
	if err != nil {
		return nil, err
	}

	if filter.Page.GetSize() < 1 || len(filter.Content) == 0 {
		return nil, fmt.Errorf("no filter %s found", name) //nolint:err113 //dynamic error is intentional
	}

	var launches openapi.PageLaunchResource
	params := ConvertToFilterParams(filter.Content[0])
	_, err = c.http.R().
		SetPathParam("project", c.project).
		SetResult(&launches).
		SetQueryParamsFromValues(params).
		Get(baseUrlLaunch)
	return &launches, err
}

// MergeLaunches merge two launches
func (c *launchClient) MergeLaunches(rq *openapi.MergeLaunchesRQ) (*openapi.LaunchResource, error) {
	var rs openapi.LaunchResource
	_, err := c.http.R().
		SetPathParam("project", c.project).
		SetBody(rq).
		SetResult(&rs).
		Post(baseUrlLaunch + "/merge")
	return &rs, err
}
