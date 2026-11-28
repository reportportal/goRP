package gorp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"resty.dev/v3"

	"github.com/reportportal/goRP/v5/pkg/openapi"
)

// ReportingClient is ReportPortal Reporting REST API Client
type ReportingClient struct {
	project string
	http    *resty.Client
}

// NewReportingClient creates new instance of ReportingClient
// host - server hostname
// project - name of the project
// apiKey - User Token (see user profile page)
func NewReportingClient(host, project string, clientOption ClientOption) *ReportingClient {
	http := resty.NewWithClient(clientOption()).
		SetBaseURL(host).
		AddResponseMiddleware(defaultHTTPErrorHandler)
	return &ReportingClient{
		project: project,
		http:    http,
	}
}

// StartLaunch starts new launch in RP
func (c *ReportingClient) StartLaunch(
	ctx context.Context,
	launch *openapi.StartLaunchRQ,
) (*openapi.EntryCreatedAsyncRS, error) {
	return c.startLaunch(ctx, launch)
}

// StartLaunchRaw starts new launch in RP with body in form of bytes buffer
func (c *ReportingClient) StartLaunchRaw(
	ctx context.Context,
	body json.RawMessage,
) (*openapi.EntryCreatedAsyncRS, error) {
	return c.startLaunch(ctx, body)
}

func (c *ReportingClient) startLaunch(
	ctx context.Context,
	body interface{},
) (*openapi.EntryCreatedAsyncRS, error) {
	var rs openapi.EntryCreatedAsyncRS
	_, err := c.http.R().
		SetContext(ctx).
		SetPathParam("project", c.project).
		SetBody(body).
		SetResult(&rs).
		Post("/api/v2/{project}/launch")

	return &rs, err
}

// FinishLaunch finishes launch in RP
func (c *ReportingClient) FinishLaunch(
	ctx context.Context,
	id string,
	launch *openapi.FinishExecutionRQ,
) (*openapi.FinishLaunchRS, error) {
	return c.finishLaunch(ctx, id, launch)
}

// FinishLaunchRaw finishes launch in RP with body in form of bytes buffer
func (c *ReportingClient) FinishLaunchRaw(
	ctx context.Context,
	id string,
	body json.RawMessage,
) (*openapi.FinishLaunchRS, error) {
	return c.finishLaunch(ctx, id, body)
}

func (c *ReportingClient) finishLaunch(
	ctx context.Context,
	id string,
	body interface{},
) (*openapi.FinishLaunchRS, error) {
	var rs openapi.FinishLaunchRS
	_, err := c.http.R().
		SetContext(ctx).
		SetPathParams(map[string]string{
			"project":  c.project,
			"launchId": id,
		}).
		SetBody(body).
		SetResult(&rs).
		Put("/api/v2/{project}/launch/{launchId}/finish")

	return &rs, err
}

// LaunchInfo is what GetLaunchByUUID tells about a launch.
type LaunchInfo struct {
	// ID is the numeric ID of the launch, as used by the UI and the non-reporting API
	ID     int64
	UUID   string
	Name   string
	Status string
	// StartTime is zero when the server did not send it
	StartTime time.Time
}

// UnmarshalJSON decodes the launch as returned by ReportPortal. The response is not decoded into the
// generated launch model on purpose: startTime is an RFC3339 string or epoch milliseconds, depending
// on the server version and the requested media type, and the generated model reads only the former.
func (l *LaunchInfo) UnmarshalJSON(data []byte) error {
	var raw struct {
		ID        int64           `json:"id"`
		UUID      string          `json:"uuid"`
		Name      string          `json:"name"`
		Status    string          `json:"status"`
		StartTime json.RawMessage `json:"startTime"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*l = LaunchInfo{ID: raw.ID, UUID: raw.UUID, Name: raw.Name, Status: raw.Status}

	var millis int64
	switch {
	case len(raw.StartTime) == 0 || string(raw.StartTime) == "null":
	case json.Unmarshal(raw.StartTime, &millis) == nil:
		l.StartTime = time.UnixMilli(millis).UTC()
	default:
		if err := json.Unmarshal(raw.StartTime, &l.StartTime); err != nil {
			return fmt.Errorf("launch startTime: %w", err)
		}
	}
	return nil
}

// GetLaunchByUUID looks a launch up by its UUID. An unknown UUID is an *HTTPError with status 404.
//
// The reporting (v2) API is asynchronous and does not check the launch UUID of an item or a log: what is
// sent for an unknown launch is accepted and dropped later. Use this call to check a UUID that was not
// returned by StartLaunch of this process before reporting into it. A launch that was started a moment
// ago may not be visible yet.
func (c *ReportingClient) GetLaunchByUUID(ctx context.Context, uuid string) (*LaunchInfo, error) {
	var launch LaunchInfo
	_, err := c.http.R().
		SetContext(ctx).
		SetPathParams(map[string]string{
			"project": c.project,
			"uuid":    uuid,
		}).
		SetResult(&launch).
		Get("/api/v1/{project}/launch/uuid/{uuid}")
	if err != nil {
		return nil, err
	}

	return &launch, nil
}

// StopLaunch finishes the launch with the STOPPED status. ReportPortal has no separate v2 stop endpoint,
// so this is FinishLaunch with Statuses.Stopped, which also lets a launch that still has running items be deleted.
func (c *ReportingClient) StopLaunch(
	ctx context.Context,
	id string,
) (*openapi.FinishLaunchRS, error) {
	return c.finishLaunch(ctx, id, &openapi.FinishExecutionRQ{
		EndTime: time.Now(),
		Status:  openapi.PtrString(string(Statuses.Stopped)),
	})
}

// StartTest starts new test in RP
func (c *ReportingClient) StartTest(
	ctx context.Context,
	item *openapi.StartTestItemRQ,
) (*openapi.EntryCreatedAsyncRS, error) {
	return c.startTest(ctx, item)
}

// StartTestRaw starts new test in RP accepting request body as array of bytes
func (c *ReportingClient) StartTestRaw(
	ctx context.Context,
	body json.RawMessage,
) (*openapi.EntryCreatedAsyncRS, error) {
	return c.startTest(ctx, body)
}

func (c *ReportingClient) startTest(
	ctx context.Context,
	body interface{},
) (*openapi.EntryCreatedAsyncRS, error) {
	var rs openapi.EntryCreatedAsyncRS
	_, err := c.http.R().
		SetContext(ctx).
		SetPathParam("project", c.project).
		SetBody(body).
		SetResult(&rs).
		Post("/api/v2/{project}/item/")

	return &rs, err
}

func (c *ReportingClient) startChildTest(
	ctx context.Context,
	parent string,
	body interface{},
) (*openapi.EntryCreatedAsyncRS, error) {
	var rs openapi.EntryCreatedAsyncRS
	_, err := c.http.R().
		SetContext(ctx).
		SetPathParams(map[string]string{
			"project": c.project,
			"itemId":  parent,
		}).
		SetBody(body).
		SetResult(&rs).
		Post("/api/v2/{project}/item/{itemId}")

	return &rs, err
}

// StartChildTest starts new test in RP
func (c *ReportingClient) StartChildTest(
	ctx context.Context,
	parent string,
	item *openapi.StartTestItemRQ,
) (*openapi.EntryCreatedAsyncRS, error) {
	return c.startChildTest(ctx, parent, item)
}

// StartChildTestRaw starts new test in RP accepting request body as array of bytes
func (c *ReportingClient) StartChildTestRaw(
	ctx context.Context,
	parent string,
	body json.RawMessage,
) (*openapi.EntryCreatedAsyncRS, error) {
	return c.startChildTest(ctx, parent, body)
}

// FinishTest finishes test in RP
func (c *ReportingClient) FinishTest(
	ctx context.Context,
	id string,
	rq *openapi.FinishTestItemRQ,
) (*openapi.OperationCompletionRS, error) {
	return c.finishTest(ctx, id, rq)
}

// FinishTestRaw finishes test in RP accepting body as array of bytes
func (c *ReportingClient) FinishTestRaw(
	ctx context.Context,
	id string,
	body json.RawMessage,
) (*openapi.OperationCompletionRS, error) {
	return c.finishTest(ctx, id, body)
}

func (c *ReportingClient) finishTest(
	ctx context.Context,
	id string,
	body interface{},
) (*openapi.OperationCompletionRS, error) {
	var rs openapi.OperationCompletionRS
	_, err := c.http.R().
		SetContext(ctx).
		SetPathParams(map[string]string{
			"project": c.project,
			"itemId":  id,
		}).
		SetBody(body).
		SetResult(&rs).
		Put("/api/v2/{project}/item/{itemId}")
	return &rs, err
}

// SaveLog attaches log in RP
func (c *ReportingClient) SaveLog(
	ctx context.Context,
	log *openapi.SaveLogRQ,
) (*openapi.EntryCreatedAsyncRS, error) {
	// A single log is sent as a one-entry multipart batch: a plain JSON body on /log is deprecated
	// in ReportPortal 6.x, while the multipart form works on every server version.
	return c.SaveLogMultipart(ctx, []*openapi.SaveLogRQ{log}, nil)
}

// SaveLogs saves logs as batch request
func (c *ReportingClient) SaveLogs(
	ctx context.Context,
	logs ...*openapi.SaveLogRQ,
) (*openapi.EntryCreatedAsyncRS, error) {
	return c.SaveLogMultipart(ctx, logs, nil)
}

// SaveLogMultipart saves a batch of logs in RP, along with any associated files (if any).
//
// Example usage:
//
// f, _ := os.Open("someFile.txt")
//
//	logs := []*SaveLogRQ{{
//			    File: FileAttachment{
//			        // note that this value must present in 'files' map as key (see below)
//			        Name: f.Name(),
//			    },
//			    LaunchUUID: launchID,
//			    ItemID:     itemID,
//			    Level:      gorp.LogLevelError,
//			    LogTime:    NewTimestamp(time.Now()),
//			    Message:    "Important message!",
//				}}
//	files:=	[]Multipart{
//					&FileMultipart{File: f},
//					&ReaderMultipart{ContentType: "text/plain", FileName: f.Name(), Reader: f}, // FileName must match the FileAttachment.Name field
//				}
//
//	 resp, err := client.SaveLogMultipart(ctx, log, files)
func (c *ReportingClient) SaveLogMultipart(
	ctx context.Context,
	log []*openapi.SaveLogRQ,
	files []Multipart,
) (*openapi.EntryCreatedAsyncRS, error) {
	var bodyBuf bytes.Buffer
	err := json.NewEncoder(&bodyBuf).Encode(log)
	if err != nil {
		return nil, fmt.Errorf("unable to encode log payload: %w", err)
	}

	rq := c.http.R().
		SetContext(ctx).
		SetPathParam("project", c.project)

	// JSON PAYLOAD PART
	rq.SetMultipartField("json_request_part", "", "application/json", &bodyBuf)

	// BINARY PART
	for _, v := range files {
		fileName, contentType, reader, lErr := v.Load()
		if lErr != nil {
			return nil, fmt.Errorf("unable to read multipart: %w", lErr)
		}
		if fileName == "" {
			return nil, errMultipartFilename
		}

		rq.SetMultipartField("file", fileName, contentType, reader)
	}

	// With attachments the server answers with a batch: {"responses":[{"id":"..."}]}.
	// The generated response type is not embedded on purpose: its custom UnmarshalJSON would be
	// promoted and swallow the "responses" field.
	var rs struct {
		ID        *string `json:"id"`
		Responses []struct {
			ID *string `json:"id"`
		} `json:"responses"`
	}
	_, err = rq.
		SetResult(&rs).
		Post("/api/v2/{project}/log")
	if err != nil {
		return nil, fmt.Errorf("unable to send log: %w", err)
	}
	if rs.ID == nil && len(rs.Responses) > 0 {
		rs.ID = rs.Responses[0].ID
	}

	return &openapi.EntryCreatedAsyncRS{Id: rs.ID}, nil
}
