package gorp

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/reportportal/goRP/v5/pkg/openapi"
)

func TestCreateRPClient(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		assert.Equal(t, "Bearer uuid", authHeader)
	}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)

	project := "prj"
	client := NewReportingClient(u.String(), project, WithApiKeyAuth(t.Context(), "uuid"))

	assert.Equal(t, "prj", client.project)
	assert.Equal(t, u.String(), client.http.BaseURL())
	_, err := client.StartLaunch(&openapi.StartLaunchRQ{})
	assert.NoError(t, err)
}

func TestHandleErrors(t *testing.T) {
	t.Parallel()
	project := "prj"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	u, _ := url.Parse(server.URL)

	client := NewClient(u, WithApiKeyAuth(t.Context(), "uuid"))
	_, _, err := client.LaunchAPI.GetProjectLaunches(t.Context(), project).Execute()
	assert.Error(t, err)
}
