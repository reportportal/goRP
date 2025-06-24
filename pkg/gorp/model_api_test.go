package gorp

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/reportportal/goRP/pkg/openapi"
)

func TestDirectionConverter(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "ASC", directionToStr(true))
	assert.Equal(t, "DESC", directionToStr(false))
}

func TestFiltersConverter(t *testing.T) {
	t.Parallel()
	fp := ConvertToFilterParams(openapi.UserFilterResource{
		Conditions: []openapi.UserFilterCondition{
			{
				FilteringField: "name",
				Condition:      "cnt",
				Value:          "value",
			},
			{
				FilteringField: "desc",
				Condition:      "eq",
				Value:          "valuedesc",
			},
		},
		Orders: []openapi.Order{
			{
				IsAsc:         false,
				SortingColumn: "name",
			},
		},
	})
	assert.Equal(t, map[string][]string{
		"filter.cnt.name": {"value"},
		"filter.eq.desc":  {"valuedesc"},
		"page.sort":       {"name,DESC"},
	}, map[string][]string(fp))
}
