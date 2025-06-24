package gorp

import (
	"fmt"
	"net/url"

	"resty.dev/v3"

	"github.com/reportportal/goRP/pkg/openapi"
)

func defaultHTTPErrorHandler(client *resty.Client, rs *resty.Response) error {
	//nolint:mnd // 4xx errors
	if (rs.StatusCode() / 100) >= 4 {
		return &HTTPError{StatusCode: rs.StatusCode(), Response: rs.String()}
	}
	return nil
}

// ConvertToFilterParams converts RP internal filter representation to query string
func ConvertToFilterParams(filter openapi.UserFilterResource) url.Values {
	params := url.Values{}
	for _, f := range filter.Conditions {
		params.Set(fmt.Sprintf("filter.%s.%s", f.Condition, f.FilteringField), f.Value)
	}

	for _, order := range filter.Orders {
		params.Set(
			"page.sort",
			fmt.Sprintf("%s,%s", order.SortingColumn, directionToStr(order.IsAsc)),
		)
	}
	return params
}

func directionToStr(asc bool) string {
	if asc {
		return "ASC"
	}
	return "DESC"
}
