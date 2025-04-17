package gorp

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateRPClient(t *testing.T) {
	t.Parallel()
	project := "prj"
	u, _ := url.Parse("http://host.com")
	client := NewClient(u, project, "uuid")

	assert.Equal(t, "prj", client.project)
	assert.Equal(t, "http://host.com", client.ReportingClient.http.BaseURL())
	assert.Equal(t, "uuid", client.ReportingClient.http.AuthToken())
}

func TestHandleErrors(t *testing.T) {
	t.Parallel()
	project := "prj"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	u, _ := url.Parse(server.URL)

	client := NewClient(u, project, "uuid")
	_, _, err := client.LaunchAPI.GetProjectLaunches(t.Context(), project).Execute()
	assert.Error(t, err)
}
