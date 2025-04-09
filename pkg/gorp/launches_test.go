package gorp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
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
					"name": "Test Launch 1"
				},
				{
					"id": 2,
					"name": "Test Launch 2"
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

	client := NewClient(server.URL, "prj", "uuid")

	result, err := client.GetLaunches()

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Content, 2)
	assert.Equal(t, int64(1), result.Content[0].ID)
	assert.Equal(t, int64(2), result.Content[1].ID)
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
					"name": "Test Launch 3"
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

	client := NewClient(server.URL, "prj", "uuid")

	paging := PageDetails{
		PageNumber: 2,
		PageSize:   10,
		SortBy:     "startTime,DESC",
	}

	result, err := client.GetLaunchesPage(paging)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Content, 1)
	assert.Equal(t, int64(3), result.Content[0].ID)
	assert.Equal(t, 2, result.Page.Number)
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
					"id": "filter1",
					"name": "test-filter",
					"conditions": [
						{
							"filteringField": "name",
							"condition": "contains",
							"value": "test"
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

	client := NewClient(server.URL, "prj", "uuid")

	result, err := client.GetFiltersByName("test-filter")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Content, 1)
	assert.Equal(t, "filter1", result.Content[0].ID)
	assert.Equal(t, "test-filter", result.Content[0].Name)
}

func TestMergeLaunches(t *testing.T) {
	t.Parallel()

	// Setup test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/prj/launch/merge", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		// Parse request body
		var rq MergeLaunchesRQ
		err := json.NewDecoder(r.Body).Decode(&rq)
		assert.NoError(t, err)
		assert.Equal(t, "Merged Launch", rq.Name)
		assert.Len(t, rq.Launches, 2)

		response := `{
			"id": 4,
			"name": "Merged Launch",
			"number": 1
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()

	client := NewClient(server.URL, "prj", "uuid")

	mergeRQ := &MergeLaunchesRQ{
		Name:        "Merged Launch",
		Description: "Merged launch description",
		Launches:    []int64{1, 2},
		MergeType:   "BASIC",
	}

	result, err := client.MergeLaunches(mergeRQ)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, int64(4), result.ID)
	assert.Equal(t, "Merged Launch", result.Name)
}
