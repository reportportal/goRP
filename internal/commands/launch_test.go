package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
)

// launchServer serves launch listing (two pages for the filter "name=big") and merge endpoints.
type launchServer struct {
	*httptest.Server

	mu         sync.Mutex
	listQuery  []string
	mergeBody  string
	mergeCalls int
}

func newLaunchServer(t *testing.T) *launchServer {
	t.Helper()

	ls := &launchServer{}
	launch := func(id int64) map[string]any {
		return map[string]any{
			"id":        id,
			"uuid":      "u" + strconv.FormatInt(id, 10),
			"name":      "launch" + strconv.FormatInt(id, 10),
			"number":    id,
			"startTime": time.Now().Format(time.RFC3339),
		}
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/proj/launch", func(w http.ResponseWriter, r *http.Request) {
		ls.mu.Lock()
		ls.listQuery = append(ls.listQuery, r.URL.RawQuery)
		ls.mu.Unlock()

		page, _ := strconv.Atoi(r.URL.Query().Get("page.page"))
		content, totalPages := []map[string]any{launch(1), launch(2)}, 1
		if r.URL.Query().Get("filter.eq.name") == "big" {
			totalPages = 2
			content = []map[string]any{launch(int64(page*2 - 1)), launch(int64(page * 2))}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"content": content,
			"page":    map[string]any{"number": page, "totalPages": totalPages},
		})
	})
	mux.HandleFunc("POST /api/v2/proj/launch/merge", func(w http.ResponseWriter, r *http.Request) {
		var body bytes.Buffer
		_, _ = body.ReadFrom(r.Body)
		ls.mu.Lock()
		ls.mergeBody = body.String()
		ls.mergeCalls++
		ls.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(launch(99))
	})
	ls.Server = httptest.NewServer(mux)
	t.Cleanup(ls.Close)

	return ls
}

// runGorp runs the launch commands under a root command that has the global connection flags.
// Not parallel-safe: HOME is replaced to hide ~/.gorp.
func runGorp(t *testing.T, srv *launchServer, args ...string) (string, error) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())

	var out bytes.Buffer
	root := &cli.Command{
		Name:   "gorp",
		Writer: &out,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "api-key"},
			&cli.StringFlag{Name: "project"},
			&cli.StringFlag{Name: "host"},
		},
		Commands:       []*cli.Command{newLaunchCommand()},
		ExitErrHandler: func(context.Context, *cli.Command, error) {},
	}
	full := append(
		[]string{"gorp", "--host", srv.URL, "--project", "proj", "--api-key", "k", "launch"},
		args...,
	)
	err := root.Run(t.Context(), full)

	return out.String(), err
}

func TestLaunchList(t *testing.T) {
	srv := newLaunchServer(t)

	out, err := runGorp(t, srv, "list")

	require.NoError(t, err)
	assert.Equal(t, "1 #1 \"launch1\"\n2 #2 \"launch2\"\n", out)
}

func TestLaunchList_FilterAndFilterNameAreExclusive(t *testing.T) {
	srv := newLaunchServer(t)

	_, err := runGorp(t, srv, "list", "--filter", "filter.eq.name=x", "--filter-name", "saved")

	require.Error(t, err)
}

func TestLaunchMerge_ByIDs(t *testing.T) {
	srv := newLaunchServer(t)

	out, err := runGorp(t, srv, "merge", "--name", "merged", "--ids", "5", "--ids", "6")

	require.NoError(t, err)
	assert.Equal(t, "99\n", out)
	assert.JSONEq(
		t,
		`{"name":"merged","mergeType":"DEEP","launches":[5,6],"extendSuitesDescription":false}`,
		srv.mergeBody,
	)
}

func TestLaunchMerge_ByFilterReadsAllPages(t *testing.T) {
	srv := newLaunchServer(t)

	_, err := runGorp(
		t,
		srv,
		"merge",
		"--name",
		"merged",
		"--type",
		"BASIC",
		"--filter",
		"filter.eq.name=big",
	)

	require.NoError(t, err)
	assert.Len(t, srv.listQuery, 2, "both pages must be requested")
	assert.JSONEq(
		t,
		`{"name":"merged","mergeType":"BASIC","launches":[1,2,3,4],"extendSuitesDescription":false}`,
		srv.mergeBody,
	)
}

func TestLaunchMerge_Validation(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "no launch selector", args: []string{"merge", "--name", "n"}},
		{
			name: "ids and filter together",
			args: []string{"merge", "--name", "n", "--ids", "1", "--filter", "x=y"},
		},
		{
			name: "unknown type",
			args: []string{"merge", "--name", "n", "--ids", "1", "--type", "WEIRD"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newLaunchServer(t)

			_, err := runGorp(t, srv, tt.args...)

			require.Error(t, err)
			assert.Zero(t, srv.mergeCalls, "nothing must be merged on invalid input")
		})
	}
}
