package gorp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"resty.dev/v3"
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
func NewReportingClient(host, project, apiKey string) *ReportingClient {
	http := resty.New().
		// SetDebug(true).
		SetBaseURL(host).
		SetAuthToken(apiKey).
		AddResponseMiddleware(defaultHTTPErrorHandler)
	return &ReportingClient{
		project: project,
		http:    http,
	}
}

// StartLaunch starts new launch in RP
func (c *ReportingClient) StartLaunch(launch *StartLaunchRQ) (*EntryCreatedRS, error) {
	return c.startLaunch(launch)
}

// StartLaunchRaw starts new launch in RP with body in form of bytes buffer
func (c *ReportingClient) StartLaunchRaw(body json.RawMessage) (*EntryCreatedRS, error) {
	return c.startLaunch(body)
}

// StartLaunch starts new launch in RP
func (c *ReportingClient) startLaunch(body interface{}) (*EntryCreatedRS, error) {
	var rs EntryCreatedRS
	_, err := c.http.R().
		SetPathParam("project", c.project).
		SetBody(body).
		SetResult(&rs).
		Post("/api/v2/{project}/launch")

	return &rs, err
}

// FinishLaunch finishes launch in RP
func (c *ReportingClient) FinishLaunch(id string, launch *FinishExecutionRQ) (*FinishLaunchRS, error) {
	return c.finishLaunch(id, launch)
}

// FinishLaunchRaw finishes launch in RP with body in form of bytes buffer
func (c *ReportingClient) FinishLaunchRaw(id string, body json.RawMessage) (*FinishLaunchRS, error) {
	return c.finishLaunch(id, body)
}

// FinishLaunch finishes launch in RP
func (c *ReportingClient) finishLaunch(id string, body interface{}) (*FinishLaunchRS, error) {
	var rs FinishLaunchRS
	_, err := c.http.R().
		SetPathParams(map[string]string{
			"project":  c.project,
			"launchId": id,
		}).
		SetBody(body).
		SetResult(&rs).
		Put("/api/v2/{project}/launch/{launchId}/finish")

	return &rs, err
}

// StopLaunch forces finishing launch
func (c *ReportingClient) StopLaunch(id string) (*MsgRS, error) {
	var rs MsgRS
	_, err := c.http.R().
		SetPathParams(map[string]string{
			"project":  c.project,
			"launchId": id,
		}).
		SetBody(&FinishExecutionRQ{
			EndTime: NewTimestamp(time.Now()),
			Status:  Statuses.Stopped,
		}).
		SetResult(&rs).
		Put("/api/v2/{project}/launch/{launchId}/stop")

	return &rs, err
}

// StartTest starts new test in RP
func (c *ReportingClient) StartTest(item *StartTestRQ) (*EntryCreatedRS, error) {
	return c.startTest(item)
}

// StartTestRaw starts new test in RP accepting request body as array of bytes
func (c *ReportingClient) StartTestRaw(body json.RawMessage) (*EntryCreatedRS, error) {
	return c.startTest(body)
}

// startTest starts new test in RP
func (c *ReportingClient) startTest(body interface{}) (*EntryCreatedRS, error) {
	var rs EntryCreatedRS
	_, err := c.http.R().
		SetPathParam("project", c.project).
		SetBody(body).
		SetResult(&rs).
		Post("/api/v2/{project}/item/")

	return &rs, err
}

// startChildTest starts new test in RP
func (c *ReportingClient) startChildTest(parent string, body interface{}) (*EntryCreatedRS, error) {
	var rs EntryCreatedRS
	_, err := c.http.R().
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
func (c *ReportingClient) StartChildTest(parent string, item *StartTestRQ) (*EntryCreatedRS, error) {
	return c.startChildTest(parent, item)
}

// StartChildTestRaw starts new test in RP accepting request body as array of bytes
func (c *ReportingClient) StartChildTestRaw(parent string, body json.RawMessage) (*EntryCreatedRS, error) {
	return c.startChildTest(parent, body)
}

// FinishTest finishes test in RP
func (c *ReportingClient) FinishTest(id string, rq *FinishTestRQ) (*MsgRS, error) {
	return c.finishTest(id, rq)
}

// FinishTestRaw finishes test in RP accepting body as array of bytes
func (c *ReportingClient) FinishTestRaw(id string, body json.RawMessage) (*MsgRS, error) {
	return c.finishTest(id, body)
}

// finishTest finishes test in RP
func (c *ReportingClient) finishTest(id string, body interface{}) (*MsgRS, error) {
	var rs MsgRS
	_, err := c.http.R().
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
func (c *ReportingClient) SaveLog(log *SaveLogRQ) (*EntryCreatedRS, error) {
	var rs EntryCreatedRS
	_, err := c.http.R().
		SetPathParam("project", c.project).
		SetBody(log).
		SetResult(&rs).
		Post("/api/v2/{project}/log")
	return &rs, err
}

// SaveLogs saves logs as batch request
func (c *ReportingClient) SaveLogs(logs ...*SaveLogRQ) (*EntryCreatedRS, error) {
	return c.SaveLogMultipart(logs, nil)
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
//	 resp, err := client.SaveLogMultipart(log, files)
func (c *ReportingClient) SaveLogMultipart(log []*SaveLogRQ, files []Multipart) (*EntryCreatedRS, error) {
	var bodyBuf bytes.Buffer
	err := json.NewEncoder(&bodyBuf).Encode(log)
	if err != nil {
		return nil, fmt.Errorf("unable to encode log payload: %w", err)
	}

	rq := c.http.R().
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

	var rs EntryCreatedRS
	_, err = rq.
		SetResult(&rs).
		Post("/api/v2/{project}/log")
	if err != nil {
		return nil, fmt.Errorf("unable to send log: %w", err)
	}
	return &rs, err
}
