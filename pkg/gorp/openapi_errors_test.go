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

// ReportPortal writes ErrorRS.errorCode as a number (see ErrorRS.ErrorTypeSerializer in service-api),
// so the generated client has to decode error bodies of this shape. The bodies below are what a real
// server returns.
func TestOpenAPIClientDecodesErrorResponse(t *testing.T) {
	t.Parallel()

	const project = "prj"

	tests := []struct {
		name   string
		status int
		body   string
		// declared tells whether the spec declares an error model for this status of the operation.
		// When it doesn't, the generated client leaves Model() empty and the caller decodes Body().
		// Such cases accept both, so a spec refresh that declares the status doesn't break the test.
		declared bool
		call     func(c *Client) error
		wantCode int64
		wantMsg  string
	}{
		{
			name:   "launch not found, 404 is not declared for the operation",
			status: http.StatusNotFound,
			body:   `{"errorCode":4041,"message":"Launch '1' not found. Did you use correct Launch ID?"}`,
			call: func(c *Client) error {
				_, _, err := c.LaunchAPI.GetLaunch(t.Context(), "1", project).Execute()

				return err
			},
			wantCode: 4041,
			wantMsg:  "Launch '1' not found. Did you use correct Launch ID?",
		},
		{
			name:     "activity not found, 404 is declared as ErrorRS",
			status:   http.StatusNotFound,
			body:     `{"errorCode":40411,"message":"Activity '1' not found. Did you use correct Activity ID?"}`,
			declared: true,
			call: func(c *Client) error {
				_, _, err := c.ActivityAPI.GetActivity(t.Context(), 1, project).Execute()

				return err
			},
			wantCode: 40411,
			wantMsg:  "Activity '1' not found. Did you use correct Activity ID?",
		},
		{
			name:     "incorrect sorting parameter, 400 is declared as ErrorRS",
			status:   http.StatusBadRequest,
			body:     `{"errorCode":40012,"message":"Sorting parameter noSuchField is not defined"}`,
			declared: true,
			call: func(c *Client) error {
				_, _, err := c.LaunchAPI.GetProjectLaunches(t.Context(), project).
					PageSort("noSuchField,asc").
					Execute()

				return err
			},
			wantCode: 40012,
			wantMsg:  "Sorting parameter noSuchField is not defined",
		},
		{
			name:     "delete launch fails on the server, 500 is declared as ErrorRS",
			status:   http.StatusInternalServerError,
			body:     `{"errorCode":5000,"message":"Unclassified error [TransientObjectException]"}`,
			declared: true,
			call: func(c *Client) error {
				_, _, err := c.LaunchAPI.DeleteLaunch(t.Context(), 80, project).Execute()

				return err
			},
			wantCode: 5000,
			wantMsg:  "Unclassified error [TransientObjectException]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srv := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(tt.status)
					w.Write([]byte(tt.body))
				}),
			)
			defer srv.Close()

			u, err := url.Parse(srv.URL)
			require.NoError(t, err)

			err = tt.call(NewClient(u, WithApiKeyAuth(t.Context(), "uuid")))
			require.Error(t, err)

			var apiErr *openapi.GenericOpenAPIError
			require.ErrorAs(t, err, &apiErr)
			assert.JSONEq(t, tt.body, string(apiErr.Body()))
			// a decoding failure would replace the HTTP status in the error text
			assert.Contains(t, err.Error(), http.StatusText(tt.status))
			assert.NotContains(t, err.Error(), "cannot unmarshal")

			errorRS, ok := apiErr.Model().(openapi.ComEpamReportportalBaseInfrastructureRulesExceptionErrorRS)
			if tt.declared {
				require.Truef(t, ok, "unexpected error model %T", apiErr.Model())
			} else if !ok {
				require.NoError(t, json.Unmarshal(apiErr.Body(), &errorRS))
			}

			assert.Equal(t, tt.wantCode, errorRS.GetErrorCode())
			assert.Equal(t, tt.wantMsg, errorRS.GetMessage())
		})
	}
}
