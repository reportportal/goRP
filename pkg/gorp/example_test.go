package gorp

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/google/uuid"

	"github.com/reportportal/goRP/v5/pkg/openapi"
)

func ExampleClient() {
	client := NewClient("", "", "")

	launchUUID := uuid.New().String()
	_, err := client.StartLaunch(&openapi.StartLaunchRQ{
		Mode:        openapi.PtrString(string(LaunchModes.Default)),
		Name:        "gorp-test",
		Uuid:        launchUUID,
		StartTime:   time.Now(),
		Description: openapi.PtrString("Demo Launch"),
	})
	checkErr(err, "unable to start launch")

	testUUID := uuid.New()
	_, err = client.StartTest(&openapi.StartTestItemRQ{
		LaunchUuid: launchUUID,
		CodeRef:    openapi.PtrString("example_test.go"),
		UniqueId:   openapi.PtrString("another one unique ID"),
		Retry:      openapi.PtrBool(false),
		Type:       string(TestItemTypes.Test),
		Name:       "Gorp Test",
		StartTime:  time.Now(),
		Uuid:       testUUID.String(),
	})
	checkErr(err, "unable to start test")

	_, err = client.SaveLog(&openapi.SaveLogRQ{
		LaunchUuid: launchUUID,
		ItemUuid:   openapi.PtrString(testUUID.String()),
		Level:      openapi.PtrString(LogLevelInfo),
		Time:       time.Now(),
		Message:    openapi.PtrString("Log without binary"),
	})
	checkErr(err, "unable to save log")

	file1, err := os.Open("../../go.mod")
	checkErr(err, "unable to read file")
	file2, err := os.Open("../../go.sum")
	checkErr(err, "unable to read file")

	_, err = client.SaveLogMultipart([]*openapi.SaveLogRQ{
		{
			LaunchUuid: launchUUID,
			ItemUuid:   openapi.PtrString(testUUID.String()),
			Level:      openapi.PtrString(LogLevelInfo),
			Message:    openapi.PtrString("Log with binary one"),
			File: &openapi.File{
				Name: openapi.PtrString("go.mod"),
			},
		},
		{
			LaunchUuid: launchUUID,
			ItemUuid:   openapi.PtrString(testUUID.String()),
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

	_, err = client.FinishTest(testUUID.String(), &openapi.FinishTestItemRQ{
		LaunchUuid: launchUUID,
		EndTime:    time.Now(),
		Status:     openapi.PtrString(string(Statuses.Passed)),
	})
	checkErr(err, "unable to finish test")

	_, err = client.FinishLaunch(launchUUID, &openapi.FinishExecutionRQ{
		Status:  openapi.PtrString(string(Statuses.Passed)),
		EndTime: time.Now(),
	})
	checkErr(err, "unable to finish launch")

	launches, err := client.GetLaunches()
	checkErr(err, "unable to get launches")
	for _, launch := range launches.Content {
		fmt.Printf("%+v\n", launch)
	}

	launchesPage, err := client.GetLaunchesPage(PageDetails{PageNumber: 1, PageSize: 50})
	checkErr(err, "unable to get launches")
	fmt.Println(len(launchesPage.Content))
	if len(launchesPage.Content) <= 2 {
		log.Fatal("expected 1 launch while getting launches page")
	}

	launchesPage, err = client.GetLaunchesByFilterPage(map[string]string{
		"launch.name": "gorp-test",
	}, PageDetails{PageNumber: 1, PageSize: 1, SortBy: "startTime,number,DESC"})
	checkErr(err, "unable to get launches")
	fmt.Println(len(launchesPage.Content))
	if len(launchesPage.Content) != 1 {
		log.Fatal("expected 1 launch while getting launches page by filter")
	}
	// Output:
}

func checkErr(err error, msg string) {
	if err != nil {
		log.Fatal(msg, err)
	}
}
