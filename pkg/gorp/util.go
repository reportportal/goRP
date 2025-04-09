package gorp

import (
	"fmt"
	"strconv"

	"resty.dev/v3"
)

func defaultHTTPErrorHandler(client *resty.Client, rs *resty.Response) error {
	//nolint:mnd // 4xx errors
	if (rs.StatusCode() / 100) >= 4 {
		return &HTTPError{StatusCode: rs.StatusCode(), Response: rs.String()}
	}
	return nil
}

// ConvertToFilterParams converts RP internal filter representation to query string
func ConvertToFilterParams(filter *FilterResource) map[string]string {
	params := map[string]string{}
	for _, f := range filter.Entities {
		params[fmt.Sprintf("filter.%s.%s", f.Condition, f.Field)] = f.Value
	}

	if filter.SelectionParams != nil {
		if filter.SelectionParams.PageNumber != 0 {
			params["page.page"] = strconv.Itoa(filter.SelectionParams.PageNumber)
		}
		if filter.SelectionParams.Orders != nil {
			for _, order := range filter.SelectionParams.Orders {
				params["page.sort"] = fmt.Sprintf("%s,%s", order.SortingColumn, directionToStr(order.Asc))
			}
		}
	}

	return params
}

func directionToStr(asc bool) string {
	if asc {
		return "ASC"
	}
	return "DESC"
}

func addPaging(details PageDetails) func(rq *resty.Request) *resty.Request {
	return func(rq *resty.Request) *resty.Request {
		if details.PageSize > 0 {
			rq.SetQueryParam("page.size", strconv.Itoa(details.PageSize))
		}
		if details.PageNumber > 0 {
			rq.SetQueryParam("page.page", strconv.Itoa(details.PageNumber))
		}
		if details.SortBy != "" {
			rq.SetQueryParam("page.sort", details.SortBy)
		}
		return rq
	}
}
