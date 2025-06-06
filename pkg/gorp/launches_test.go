package gorp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/reportportal/goRP/v5/pkg/openapi"
)

func TestGetLaunches(t *testing.T) {
	t.Parallel()

	// Setup test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/prj/launch", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)

		response := `{
			"content": [
				{
					"id": 1,
					"name": "Test Launch 1",
					"uuid": "014b329b-a882-4c2d-9988-c2f6179a421b",
					"number": 1,
					"startTime": "2025-02-21T16:30:42.673Z",
					"status": "PASSED"
				},
				{
					"id": 2,
					"name": "Test Launch 2",
					"uuid": "014b329b-a882-4c2d-9988-c2f6179a421c",
					"number": 2,
					"startTime": "2025-02-21T16:30:42.673Z",
					"status": "PASSED"
				}
			],
			"page": {
				"number": 1,
				"size": 2,
				"totalElements": 2,
				"totalPages": 1
			}
		}`
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()
	u, _ := url.Parse(server.URL)

	client := NewClient(u, "uuid")

	result, _, err := client.LaunchAPI.GetProjectLaunches(t.Context(), "prj").Execute()

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Content, 2)
	assert.Equal(t, int64(1), result.Content[0].Id)
	assert.Equal(t, int64(2), result.Content[1].Id)
}

func TestGetLaunchesPage(t *testing.T) {
	t.Parallel()

	// Setup test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/prj/launch", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)

		// Check page parameters
		assert.Equal(t, "10", r.URL.Query().Get("page.size"))
		assert.Equal(t, "2", r.URL.Query().Get("page.page"))
		assert.Equal(t, "startTime,DESC", r.URL.Query().Get("page.sort"))

		response := `{
			"content": [
				{
					"id": 3,
					"name": "Test Launch 3",
					"uuid": "014b329b-a882-4c2d-9988-c2f6179a421b",
					"number": 1,
					"startTime": "2025-02-21T16:30:42.673Z",
					"status": "PASSED"
				}
			],
			"page": {
				"number": 2,
				"size": 10,
				"totalElements": 11,
				"totalPages": 2
			}
		}`
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()

	u, _ := url.Parse(server.URL)
	client := NewClient(u, "uuid")

	result, _, err := client.LaunchAPI.GetProjectLaunches(t.Context(), "prj").
		PagePage(2).
		PageSize(10).
		PageSort("startTime,DESC").
		Execute()

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Len(t, result.Content, 1)
	assert.Equal(t, int64(3), result.Content[0].Id)
	assert.Equal(t, int64(2), *result.Page.Number)
}

func TestGetFiltersByName(t *testing.T) {
	t.Parallel()

	// Setup test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/prj/filter", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "test-filter", r.URL.Query().Get("filter.eq.name"))

		response := `{
			"content": [
				{
					"id": 1,
					"name": "test-filter",
					"owner": "some owner",
					"type": "Launch",
					"conditions": [
						{
							"filteringField": "name",
							"condition": "contains",
							"value": "test"
						}
					],
					"orders": [
						{
							"sortingColumn": "startTime",
							"isAsc": true	
						}
					]
				}
			],
			"page": {
				"number": 1,
				"size": 1,
				"totalElements": 1,
				"totalPages": 1
			}
		}`
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()

	u, _ := url.Parse(server.URL)
	client := NewClient(u, "uuid")

	result, _, err := client.UserFilterAPI.GetAllFilters(t.Context(), "prj").
		FilterEqName("test-filter").
		Execute()

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Content, 1)
	assert.Equal(t, int64(1), result.Content[0].Id)
	assert.Equal(t, "test-filter", result.Content[0].Name)
}

func TestMergeLaunches(t *testing.T) {
	t.Parallel()

	// Setup test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v2/prj/launch/merge", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		// Parse request body
		var rq openapi.MergeLaunchesRQ
		err := json.NewDecoder(r.Body).Decode(&rq)
		assert.NoError(t, err)
		assert.Equal(t, "Merged Launch", rq.Name)
		assert.Len(t, rq.Launches, 2)

		response := `{
			"id": 4,
			"name": "Merged Launch",
			"number": 1,
			"uuid": "014b329b-a882-4c2d-9988-c2f6179a421b",
			"startTime": "2025-02-21T16:30:42.673Z",
			"status": "PASSED"

		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()

	u, _ := url.Parse(server.URL)
	client := NewClient(u, "uuid")

	mergeRQ := openapi.MergeLaunchesRQ{
		Name:        "Merged Launch",
		Description: openapi.PtrString("Merged launch description"),
		Launches:    []int64{1, 2},
		MergeType:   "BASIC",
	}

	result, _, err := client.LaunchAsyncAPI.MergeLaunchesOldUuid(t.Context(), "prj").
		MergeLaunchesRQ(mergeRQ).
		Execute()

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, int64(4), result.Id)
	assert.Equal(t, "Merged Launch", result.Name)
}
