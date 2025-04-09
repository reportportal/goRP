package gorp

import (
	"fmt"

	"resty.dev/v3"
)

type launchClient struct {
	project string
	http    *resty.Client
}

// GetLaunches retrieves latest launches
func (c *launchClient) GetLaunches() (*LaunchPage, error) {
	var launches LaunchPage
	_, err := c.http.R().
		SetPathParam("project", c.project).
		SetResult(&launches).
		Get(baseUrlLaunch)
	return &launches, err
}

// GetLaunchesPage retrieves latest launches with paging
func (c *launchClient) GetLaunchesPage(paging PageDetails) (*LaunchPage, error) {
	var launches LaunchPage
	_, err := c.http.R().
		SetPathParam("project", c.project).
		SetResult(&launches).
		Funcs(addPaging(paging)).
		Get(baseUrlLaunch)
	return &launches, err
}

// GetLaunchesByFilter retrieves launches by filter
func (c *launchClient) GetLaunchesByFilter(filter map[string]string) (*LaunchPage, error) {
	var launches LaunchPage
	_, err := c.http.R().
		SetPathParam("project", c.project).
		SetResult(&launches).
		SetQueryParams(filter).
		Get(baseUrlLaunch)
	return &launches, err
}

// GetLaunchesByFilterPage retrieves launches by filter with paging
func (c *launchClient) GetLaunchesByFilterPage(filter map[string]string, paging PageDetails) (*LaunchPage, error) {
	var launches LaunchPage
	_, err := c.http.R().
		SetPathParam("project", c.project).
		SetResult(&launches).
		SetQueryParams(filter).
		Funcs(addPaging(paging)).
		Get(baseUrlLaunch)
	return &launches, err
}

// GetLaunchesByFilterString retrieves launches by filter as string
func (c *launchClient) GetLaunchesByFilterString(filter string) (*LaunchPage, error) {
	var launches LaunchPage
	_, err := c.http.R().
		SetPathParam("project", c.project).
		SetResult(&launches).
		SetQueryString(filter).
		Get(baseUrlLaunch)
	return &launches, err
}

// GetLaunchesByFilterName retrieves launches by filter name
func (c *launchClient) GetLaunchesByFilterName(name string) (*LaunchPage, error) {
	filter, err := (&filterClient{project: c.project, http: c.http}).GetFiltersByName(name)
	if err != nil {
		return nil, err
	}

	if filter.Page.Size < 1 || len(filter.Content) == 0 {
		return nil, fmt.Errorf("no filter %s found", name) //nolint:err113 //dynamic error is intentional
	}

	var launches LaunchPage
	params := ConvertToFilterParams(filter.Content[0])
	_, err = c.http.R().
		SetPathParam("project", c.project).
		SetResult(&launches).
		SetQueryParams(params).
		Get(baseUrlLaunch)
	return &launches, err
}

// MergeLaunches merge two launches
func (c *launchClient) MergeLaunches(rq *MergeLaunchesRQ) (*LaunchResource, error) {
	var rs LaunchResource
	_, err := c.http.R().
		SetPathParam("project", c.project).
		SetBody(rq).
		SetResult(&rs).
		Post(baseUrlLaunch + "/merge")
	return &rs, err
}
