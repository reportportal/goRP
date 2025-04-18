package gorp

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/google/uuid"
)

func ExampleClient() {
	client := NewClient("", "", "")

	launchUUID := uuid.New()
	launch, err := client.StartLaunch(&StartLaunchRQ{
		Mode: LaunchModes.Default,
		StartRQ: StartRQ{
			Name:        "gorp-test",
			UUID:        &launchUUID,
			StartTime:   NewTimestamp(time.Now()),
			Description: "Demo Launch",
		},
	})
	checkErr(err, "unable to start launch")

	testUUID := uuid.New()
	_, err = client.StartTest(&StartTestRQ{
		LaunchID: launch.ID,
		CodeRef:  "example_test.go",
		UniqueID: "another one unique ID",
		Retry:    false,
		Type:     TestItemTypes.Test,
		StartRQ: StartRQ{
			Name:      "Gorp Test",
			StartTime: Timestamp{time.Now()},
			UUID:      &testUUID,
		},
	})
	checkErr(err, "unable to start test")

	_, err = client.SaveLog(&SaveLogRQ{
		LaunchUUID: launchUUID.String(),
		ItemID:     testUUID.String(),
		Level:      LogLevelInfo,
		LogTime:    Timestamp{time.Now()},
		Message:    "Log without binary",
	})
	checkErr(err, "unable to save log")

	file1, err := os.Open("../../go.mod")
	checkErr(err, "unable to read file")
	file2, err := os.Open("../../go.sum")
	checkErr(err, "unable to read file")

	_, err = client.SaveLogMultipart([]*SaveLogRQ{
		{
			LaunchUUID: launchUUID.String(),
			ItemID:     testUUID.String(),
			Level:      LogLevelInfo,
			Message:    "Log with binary one",
			Attachment: Attachment{
				Name: "go.mod",
			},
		},
		{
			LaunchUUID: launchUUID.String(),
			ItemID:     testUUID.String(),
			Level:      LogLevelInfo,
			Message:    "Log with binary two",
			Attachment: Attachment{
				Name: "go.sum",
			},
		},
	}, []Multipart{
		&FileMultipart{File: file1},
		&ReaderMultipart{ContentType: "text/plain", FileName: file2.Name(), Reader: file2},
	})

	checkErr(err, "unable to save log multipart")

	_, err = client.FinishTest(testUUID.String(), &FinishTestRQ{
		LaunchUUID: launchUUID.String(),
		FinishExecutionRQ: FinishExecutionRQ{
			EndTime: Timestamp{time.Now()},
			Status:  Statuses.Passed,
		},
	})
	checkErr(err, "unable to finish test")

	_, err = client.FinishLaunch(launchUUID.String(), &FinishExecutionRQ{
		Status:  Statuses.Passed,
		EndTime: Timestamp{time.Now()},
	})
	checkErr(err, "unable to finish launch")

	launches, err := client.GetLaunches()
	checkErr(err, "unable to get launches")
	for _, launch := range launches.Content {
		fmt.Printf("%+v\n", launch)
	}

	launchesPage, err := client.GetLaunchesPage(PageDetails{PageNumber: 1, PageSize: 1})
	checkErr(err, "unable to get launches")
	if len(launchesPage.Content) != 1 {
		log.Fatal("expected 1 launch")
	}
}

func checkErr(err error, msg string) {
	if err != nil {
		log.Fatal(msg, err)
	}
}
