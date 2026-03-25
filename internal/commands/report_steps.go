package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/urfave/cli/v3"

	gorppkg "github.com/reportportal/goRP/v5/pkg/gorp"
	"github.com/reportportal/goRP/v5/pkg/openapi"
)

var (
	startLaunchCommand = &cli.Command{
		Name:  "start-launch",
		Usage: "Start a new launch and print its UUID to stdout",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "name",
				Aliases:  []string{"n"},
				Usage:    "Launch name",
				Required: true,
			},
			&cli.StringFlag{
				Name:  "description",
				Usage: "Launch description",
			},
			&cli.StringSliceFlag{
				Name:    "attr",
				Aliases: []string{"a"},
				Usage:   "Launch attribute with format 'key:value'. Omitting ':' tags the launch with just the value.",
			},
			&cli.StringFlag{
				Name:  "mode",
				Usage: "Launch mode: DEFAULT or DEBUG",
				Value: "DEFAULT",
			},
		},
		Action: startLaunchAction,
	}

	startTestCommand = &cli.Command{
		Name:  "start-test",
		Usage: "Start a test item and print its UUID to stdout",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "launch-uuid",
				Usage:    "Launch UUID",
				Sources:  cli.EnvVars("LAUNCH_UUID"),
				Required: true,
			},
			&cli.StringFlag{
				Name:     "name",
				Aliases:  []string{"n"},
				Usage:    "Test item name",
				Required: true,
			},
			&cli.StringFlag{
				Name:    "type",
				Aliases: []string{"t"},
				Usage:   "Test item type: SUITE, TEST, STEP, SCENARIO, etc.",
				Value:   "TEST",
			},
			&cli.StringFlag{
				Name:    "parent-uuid",
				Usage:   "Parent test item UUID (creates a child item when set)",
				Sources: cli.EnvVars("PARENT_UUID"),
			},
			&cli.StringFlag{
				Name:  "description",
				Usage: "Test item description",
			},
			&cli.StringFlag{
				Name:  "code-ref",
				Usage: "Code reference (e.g. package/TestName)",
			},
			&cli.StringSliceFlag{
				Name:    "attr",
				Aliases: []string{"a"},
				Usage:   "Test item attribute with format 'key:value'",
			},
		},
		Action: startTestAction,
	}

	reportLogCommand = &cli.Command{
		Name:  "log",
		Usage: "Report a log entry, optionally with a file attachment",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "launch-uuid",
				Usage:    "Launch UUID",
				Sources:  cli.EnvVars("LAUNCH_UUID"),
				Required: true,
			},
			&cli.StringFlag{
				Name:    "item-uuid",
				Usage:   "Test item UUID",
				Sources: cli.EnvVars("ITEM_UUID"),
			},
			&cli.StringFlag{
				Name:     "message",
				Aliases:  []string{"m"},
				Usage:    "Log message text",
				Required: true,
			},
			&cli.StringFlag{
				Name:  "level",
				Usage: "Log level: DEBUG, INFO, or ERROR",
				Value: "INFO",
			},
			&cli.StringFlag{
				Name:    "file",
				Aliases: []string{"f"},
				Usage:   "Path to a file to attach to the log entry",
			},
		},
		Action: reportLogAction,
	}

	finishTestCommand = &cli.Command{
		Name:  "finish-test",
		Usage: "Finish a test item",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "launch-uuid",
				Usage:    "Launch UUID",
				Sources:  cli.EnvVars("LAUNCH_UUID"),
				Required: true,
			},
			&cli.StringFlag{
				Name:     "item-uuid",
				Usage:    "Test item UUID",
				Sources:  cli.EnvVars("ITEM_UUID"),
				Required: true,
			},
			&cli.StringFlag{
				Name:  "status",
				Usage: "Test item status: PASSED, FAILED, SKIPPED, etc.",
			},
		},
		Action: finishTestAction,
	}

	finishLaunchCommand = &cli.Command{
		Name:  "finish-launch",
		Usage: "Finish a launch",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "launch-uuid",
				Usage:    "Launch UUID",
				Sources:  cli.EnvVars("LAUNCH_UUID"),
				Required: true,
			},
			&cli.StringFlag{
				Name:  "status",
				Usage: "Launch status: PASSED, FAILED, STOPPED, etc.",
			},
		},
		Action: finishLaunchAction,
	}
)

func parseAttributes(attrArgs []string) []openapi.ItemAttributesRQ {
	if len(attrArgs) == 0 {
		return nil
	}
	attrs := make([]openapi.ItemAttributesRQ, len(attrArgs))
	for i, raw := range attrArgs {
		attr := openapi.ItemAttributesRQ{System: openapi.PtrBool(false)}
		if key, value, ok := strings.Cut(raw, ":"); ok {
			attr.SetKey(key)
			attr.SetValue(value)
		} else {
			attr.SetValue(raw)
		}
		attrs[i] = attr
	}
	return attrs
}

// ---------------------------------------------------------------------------
// CLI action wrappers — extract config from flags, delegate to step functions
// ---------------------------------------------------------------------------

func startLaunchAction(ctx context.Context, cmd *cli.Command) error {
	cfg, err := getConfig(cmd)
	if err != nil {
		return err
	}
	_, err = startLaunchStep(ctx, cfg, cmd)
	return err
}

func startTestAction(ctx context.Context, cmd *cli.Command) error {
	cfg, err := getConfig(cmd)
	if err != nil {
		return err
	}
	_, err = startTestStep(ctx, cfg, cmd)
	return err
}

func reportLogAction(ctx context.Context, cmd *cli.Command) error {
	cfg, err := getConfig(cmd)
	if err != nil {
		return err
	}
	_, err = reportLogStep(ctx, cfg, cmd)
	return err
}

func finishTestAction(ctx context.Context, cmd *cli.Command) error {
	cfg, err := getConfig(cmd)
	if err != nil {
		return err
	}
	return finishTestStep(ctx, cfg, cmd)
}

func finishLaunchAction(ctx context.Context, cmd *cli.Command) error {
	cfg, err := getConfig(cmd)
	if err != nil {
		return err
	}
	return finishLaunchStep(ctx, cfg, cmd)
}

// ---------------------------------------------------------------------------
// Step functions — testable core logic
// ---------------------------------------------------------------------------

func startLaunchStep(ctx context.Context, cfg *clientConfig, cmd *cli.Command) (string, error) {
	client := buildReportingClient(ctx, cfg)

	rq := &openapi.StartLaunchRQ{
		Name:      cmd.String("name"),
		StartTime: time.Now(),
		Mode:      openapi.PtrString(cmd.String("mode")),
	}
	if desc := cmd.String("description"); desc != "" {
		rq.Description = openapi.PtrString(desc)
	}
	if attrs := parseAttributes(cmd.StringSlice("attr")); len(attrs) > 0 {
		rq.Attributes = attrs
	}

	rs, err := client.StartLaunch(ctx, rq)
	if err != nil {
		return "", err
	}

	_, _ = fmt.Fprintln(cmd.Writer, *rs.Id)

	return *rs.Id, nil
}

func startTestStep(ctx context.Context, cfg *clientConfig, cmd *cli.Command) (string, error) {
	client := buildReportingClient(ctx, cfg)

	rq := &openapi.StartTestItemRQ{
		LaunchUuid: cmd.String("launch-uuid"),
		Name:       cmd.String("name"),
		Type:       cmd.String("type"),
		StartTime:  time.Now(),
		HasStats:   openapi.PtrBool(true),
		Retry:      openapi.PtrBool(false),
	}
	if desc := cmd.String("description"); desc != "" {
		rq.Description = openapi.PtrString(desc)
	}
	if codeRef := cmd.String("code-ref"); codeRef != "" {
		rq.CodeRef = openapi.PtrString(codeRef)
	}
	if attrs := parseAttributes(cmd.StringSlice("attr")); len(attrs) > 0 {
		rq.Attributes = attrs
	}

	var rs *openapi.EntryCreatedAsyncRS
	var err error
	if parentUUID := cmd.String("parent-uuid"); parentUUID != "" {
		rs, err = client.StartChildTest(ctx, parentUUID, rq)
	} else {
		rs, err = client.StartTest(ctx, rq)
	}
	if err != nil {
		return "", err
	}

	_, _ = fmt.Fprintln(cmd.Writer, *rs.Id)

	return *rs.Id, nil
}

func reportLogStep(ctx context.Context, cfg *clientConfig, cmd *cli.Command) (string, error) {
	client := buildReportingClient(ctx, cfg)

	logRQ := &openapi.SaveLogRQ{
		LaunchUuid: cmd.String("launch-uuid"),
		Level:      openapi.PtrString(cmd.String("level")),
		Time:       time.Now(),
		Message:    openapi.PtrString(cmd.String("message")),
	}
	if itemUUID := cmd.String("item-uuid"); itemUUID != "" {
		logRQ.ItemUuid = openapi.PtrString(itemUUID)
	}

	var (
		rs  *openapi.EntryCreatedAsyncRS
		err error
	)

	if filePath := cmd.String("file"); filePath != "" {
		rs, err = reportLogWithFile(ctx, client, logRQ, filePath)
	} else {
		rs, err = client.SaveLog(ctx, logRQ)
	}

	if err != nil {
		return "", err
	}

	_, _ = fmt.Fprintln(cmd.Writer, *rs.Id)

	return *rs.Id, nil
}

func reportLogWithFile(
	ctx context.Context,
	client *gorppkg.ReportingClient,
	logRQ *openapi.SaveLogRQ,
	filePath string,
) (*openapi.EntryCreatedAsyncRS, error) {
	cleanPath := filepath.Clean(filePath)

	f, err := os.Open(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("unable to open file %s: %w", filePath, err)
	}
	defer func() { _ = f.Close() }()

	logRQ.File = &openapi.File{
		Name: openapi.PtrString(filepath.Base(cleanPath)),
	}

	return client.SaveLogMultipart(ctx, []*openapi.SaveLogRQ{logRQ}, []gorppkg.Multipart{
		&gorppkg.FileMultipart{File: f},
	})
}

func finishTestStep(ctx context.Context, cfg *clientConfig, cmd *cli.Command) error {
	client := buildReportingClient(ctx, cfg)

	rq := &openapi.FinishTestItemRQ{
		LaunchUuid: cmd.String("launch-uuid"),
		EndTime:    time.Now(),
	}
	if status := cmd.String("status"); status != "" {
		rq.Status = openapi.PtrString(status)
	}

	_, err := client.FinishTest(ctx, cmd.String("item-uuid"), rq)
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintln(cmd.Writer, cmd.String("item-uuid"))

	return nil
}

func finishLaunchStep(ctx context.Context, cfg *clientConfig, cmd *cli.Command) error {
	client := buildReportingClient(ctx, cfg)

	rq := &openapi.FinishExecutionRQ{
		EndTime: time.Now(),
	}
	if status := cmd.String("status"); status != "" {
		rq.Status = openapi.PtrString(status)
	}

	_, err := client.FinishLaunch(ctx, cmd.String("launch-uuid"), rq)
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintln(cmd.Writer, cmd.String("launch-uuid"))

	return nil
}
