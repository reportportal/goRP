package gorp

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"time"

	"github.com/reportportal/goRP/v5/pkg/openapi"
)

func ExampleClient() {
	defaultProject := ""
	clientOpt := WithApiKeyAuth(context.Background(), "")
	client := NewClient(&url.URL{}, clientOpt)
	reportingClient := NewReportingClient("", defaultProject, clientOpt)

	rs, err := reportingClient.StartLaunch(&openapi.StartLaunchRQ{
		Mode:        openapi.PtrString(string(LaunchModes.Default)),
		Name:        "gorp-test",
		StartTime:   time.Now(),
		Description: openapi.PtrString("Demo Launch"),
	})
	checkErr(err, "unable to start launch")
	launchUUID := *rs.Id

	testRS, err := reportingClient.StartTest(&openapi.StartTestItemRQ{
		LaunchUuid: launchUUID,
		CodeRef:    openapi.PtrString("example_test.go"),
		UniqueId:   openapi.PtrString("another one unique ID"),
		Retry:      openapi.PtrBool(false),
		Type:       string(TestItemTypes.Test),
		Name:       "Gorp Test",
		StartTime:  time.Now(),
	})
	checkErr(err, "unable to start test")

	testUUID := *testRS.Id
	_, err = reportingClient.SaveLog(&openapi.SaveLogRQ{
		LaunchUuid: launchUUID,
		ItemUuid:   openapi.PtrString(testUUID),
		Level:      openapi.PtrString(LogLevelInfo),
		Time:       time.Now(),
		Message:    openapi.PtrString("Log without binary"),
	})
	checkErr(err, "unable to save log")

	file1, err := os.Open("../../go.mod")
	checkErr(err, "unable to read file")
	file2, err := os.Open("../../go.sum")
	checkErr(err, "unable to read file")

	_, err = reportingClient.SaveLogMultipart([]*openapi.SaveLogRQ{
		{
			LaunchUuid: launchUUID,
			ItemUuid:   openapi.PtrString(testUUID),
			Level:      openapi.PtrString(LogLevelInfo),
			Message:    openapi.PtrString("Log with binary one"),
			File: &openapi.File{
				Name: openapi.PtrString("go.mod"),
			},
		},
		{
			LaunchUuid: launchUUID,
			ItemUuid:   openapi.PtrString(testUUID),
			Level:      openapi.PtrString(LogLevelInfo),
			Message:    openapi.PtrString("Log with binary two"),
			File: &openapi.File{
				Name: openapi.PtrString("go.sum"),
			},
		},
	}, []Multipart{
		&FileMultipart{File: file1},
		&ReaderMultipart{ContentType: "text/plain", FileName: file2.Name(), Reader: file2},
	})

	checkErr(err, "unable to save log multipart")

	_, err = reportingClient.FinishTest(testUUID, &openapi.FinishTestItemRQ{
		LaunchUuid: launchUUID,
		EndTime:    time.Now(),
		Status:     openapi.PtrString(string(Statuses.Passed)),
	})
	checkErr(err, "unable to finish test")

	_, err = reportingClient.FinishLaunch(launchUUID, &openapi.FinishExecutionRQ{
		Status:  openapi.PtrString(string(Statuses.Passed)),
		EndTime: time.Now(),
	})
	checkErr(err, "unable to finish launch")

	launches, _, err := client.LaunchAPI.GetProjectLaunches(context.Background(), defaultProject).
		Execute()
	checkErr(err, "unable to get launches")
	for _, launch := range launches.Content {
		fmt.Printf("%+v\n", launch)
	}

	launchesPage, _, err := client.LaunchAPI.GetProjectLaunches(context.Background(), defaultProject).
		PagePage(1).
		PageSize(50).
		Execute()
	checkErr(err, "unable to get launches")
	fmt.Println(len(launchesPage.Content))
	if len(launchesPage.Content) <= 2 {
		log.Fatal("expected 1 launch while getting launches page")
	}

	launchesPage, _, err = client.LaunchAPI.GetProjectLaunches(context.Background(), defaultProject).
		FilterEqName("gorp-test").
		PagePage(1).
		PageSize(1).
		PageSort("startTime,number,DESC").
		Execute()

	checkErr(err, "unable to get launches")
	fmt.Println(len(launchesPage.Content))
	if len(launchesPage.Content) != 1 {
		log.Fatal("expected 1 launch while getting launches page by filter")
	}
}

func checkErr(err error, msg string) {
	if err != nil {
		log.Fatal(msg, err)
	}
}
