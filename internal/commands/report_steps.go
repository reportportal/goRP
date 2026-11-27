package commands

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
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
			attrFlag(
				"Launch attribute with format 'key:value'. Omitting ':' tags the launch with just the value.",
			),
			&cli.StringFlag{
				Name:  "mode",
				Usage: "Launch mode: DEFAULT or DEBUG",
				Value: "DEFAULT",
			},
			&cli.BoolFlag{
				Name: "rerun",
				Usage: "Rerun a launch instead of starting a new one: the latest launch with the same name, " +
					"or the one given by --rerun-of. Prints the UUID of the launch being rerun.",
			},
			&cli.StringFlag{
				Name:  "rerun-of",
				Usage: "UUID of the launch to rerun (implies --rerun)",
			},
		},
		Action: withConfig(startLaunchStep),
	}

	startTestCommand = &cli.Command{
		Name:  "start-test",
		Usage: "Start a test item and print its UUID to stdout",
		Flags: []cli.Flag{
			launchUUIDFlag(),
			&cli.StringFlag{
				Name:     "name",
				Aliases:  []string{"n"},
				Usage:    "Test item name",
				Required: true,
			},
			&cli.StringFlag{
				Name:    "type",
				Aliases: []string{"t"},
				Usage:   "Test item type: " + strings.Join(testItemTypes(), ", "),
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
			attrFlag("Test item attribute with format 'key:value'"),
			&cli.StringFlag{
				Name:  "test-case-id",
				Usage: "Test case ID: identifies the same test across launches (by default the server derives it from --code-ref)",
			},
			&cli.StringSliceFlag{
				Name:  "param",
				Usage: "Test parameter with format 'key=value'. Repeatable.",
			},
			&cli.StringFlag{
				Name:  "retry-of",
				Usage: "UUID of the previous attempt: reports this item as its retry. Requires --parent-uuid.",
			},
			&cli.BoolFlag{
				Name:  "has-stats",
				Usage: "Count the item in statistics. Use --has-stats=false for a nested step.",
				Value: true,
			},
		},
		Action: withConfig(startTestStep),
	}

	reportLogCommand = &cli.Command{
		Name:  "log",
		Usage: "Report a log entry, optionally with a file attachment",
		Flags: []cli.Flag{
			launchUUIDFlag(),
			itemUUIDFlag(false),
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
		Action: withConfig(reportLogStep),
	}

	finishTestCommand = &cli.Command{
		Name:  "finish-test",
		Usage: "Finish a test item",
		Flags: []cli.Flag{
			launchUUIDFlag(),
			itemUUIDFlag(true),
			statusFlag("Test item status: PASSED, FAILED, SKIPPED, etc."),
			&cli.StringFlag{
				Name: "issue-type",
				Usage: "Defect type locator (e.g. pb001, ab001, si001, ti001, nd001, or a custom one) or NOT_ISSUE. " +
					"The server applies it only to FAILED or SKIPPED items.",
			},
			&cli.StringFlag{
				Name:  "description",
				Usage: "Test item description (replaces the one given at start)",
			},
			attrFlag(
				"Test item attribute with format 'key:value', added to the ones given at start",
			),
		},
		Action: withConfig(adaptNoResult(finishTestStep)),
	}

	finishLaunchCommand = &cli.Command{
		Name:  "finish-launch",
		Usage: "Finish a launch",
		Flags: []cli.Flag{
			launchUUIDFlag(),
			statusFlag("Launch status: PASSED, FAILED, STOPPED, etc."),
			&cli.StringFlag{
				Name:  "description",
				Usage: "Text appended to the launch description given at start",
			},
			attrFlag("Launch attribute with format 'key:value', added to the ones given at start"),
		},
		Action: withConfig(adaptNoResult(finishLaunchStep)),
	}
)

// adaptNoResult lets steps that only return an error be used with withConfig.
func adaptNoResult(
	step func(ctx context.Context, cfg *clientConfig, cmd *cli.Command) error,
) stepFunc[struct{}] {
	return func(ctx context.Context, cfg *clientConfig, cmd *cli.Command) (struct{}, error) {
		return struct{}{}, step(ctx, cfg, cmd)
	}
}

func launchModes() []string {
	return []string{string(gorppkg.LaunchModes.Default), string(gorppkg.LaunchModes.Debug)}
}

func testItemTypes() []string {
	t := gorppkg.TestItemTypes
	types := []gorppkg.TestItemType{
		t.Suite, t.Story, t.Test, t.Scenario, t.Step,
		t.BeforeClass, t.BeforeGroups, t.BeforeMethod, t.BeforeSuite, t.BeforeTest,
		t.AfterClass, t.AfterGroups, t.AfterMethod, t.AfterSuite, t.AfterTest,
	}
	names := make([]string, len(types))
	for i, itemType := range types {
		names[i] = string(itemType)
	}

	return names
}

// parseEnumFlag returns the value in upper case if it is one of the allowed values, ignoring case.
func parseEnumFlag(flag, value string, allowed []string) (string, error) {
	upper := strings.ToUpper(strings.TrimSpace(value))
	if !slices.Contains(allowed, upper) {
		return "", fmt.Errorf(
			"invalid --%s %q: must be one of %s",
			flag,
			value,
			strings.Join(allowed, ", "),
		)
	}

	return upper, nil
}

// parseParameters converts 'key=value' arguments to test item parameters. The value may be empty and
// may contain '='; the key is required.
func parseParameters(
	paramArgs []string,
) ([]openapi.ComEpamReportportalBaseReportingParameterResource, error) {
	if len(paramArgs) == 0 {
		return nil, nil
	}
	params := make([]openapi.ComEpamReportportalBaseReportingParameterResource, len(paramArgs))
	for i, raw := range paramArgs {
		key, value, ok := strings.Cut(raw, "=")
		if !ok || key == "" {
			return nil, fmt.Errorf("invalid --param %q: expected 'key=value'", raw)
		}
		params[i] = openapi.ComEpamReportportalBaseReportingParameterResource{
			Key:   key,
			Value: openapi.PtrString(value),
		}
	}

	return params, nil
}

// ---------------------------------------------------------------------------
// Step functions — testable core logic
// ---------------------------------------------------------------------------

func startLaunchStep(ctx context.Context, cfg *clientConfig, cmd *cli.Command) (string, error) {
	mode, err := parseEnumFlag("mode", cmd.String("mode"), launchModes())
	if err != nil {
		return "", err
	}

	client := buildReportingClient(ctx, cfg)

	rq := &openapi.StartLaunchRQ{
		Name:      cmd.String("name"),
		StartTime: time.Now(),
		Mode:      openapi.PtrString(mode),
	}
	// The server ignores rerunOf unless rerun is set, so --rerun-of alone would silently start a new launch.
	if rerunOf := cmd.String("rerun-of"); rerunOf != "" {
		rq.Rerun = openapi.PtrBool(true)
		rq.RerunOf = openapi.PtrString(rerunOf)
	} else if cmd.Bool("rerun") {
		rq.Rerun = openapi.PtrBool(true)
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

	return printEntryID(cmd, rs)
}

func startTestStep(ctx context.Context, cfg *clientConfig, cmd *cli.Command) (string, error) {
	if args := cmd.Args(); args != nil && args.Present() {
		// "--has-stats false" leaves "false" as an argument and would report the item with statistics.
		return "", fmt.Errorf(
			"unexpected argument %q: a boolean flag takes its value with '=', e.g. --has-stats=false",
			args.First(),
		)
	}
	itemType, err := parseEnumFlag("type", cmd.String("type"), testItemTypes())
	if err != nil {
		return "", err
	}
	params, err := parseParameters(cmd.StringSlice("param"))
	if err != nil {
		return "", err
	}
	parentUUID := cmd.String("parent-uuid")
	retryOf := cmd.String("retry-of")
	if retryOf != "" && parentUUID == "" {
		// The server refuses a root retry, but only after the asynchronous API has answered 201.
		return "", errors.New("--retry-of requires --parent-uuid: a root item can't be a retry")
	}

	client := buildReportingClient(ctx, cfg)

	rq := &openapi.StartTestItemRQ{
		LaunchUuid: cmd.String("launch-uuid"),
		Name:       cmd.String("name"),
		Type:       itemType,
		StartTime:  time.Now(),
		HasStats:   openapi.PtrBool(cmd.Bool("has-stats")),
		Retry:      openapi.PtrBool(retryOf != ""),
		Parameters: params,
	}
	if retryOf != "" {
		rq.RetryOf = openapi.PtrString(retryOf)
	}
	if testCaseID := cmd.String("test-case-id"); testCaseID != "" {
		rq.TestCaseId = openapi.PtrString(testCaseID)
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
	if parentUUID != "" {
		rs, err = client.StartChildTest(ctx, parentUUID, rq)
	} else {
		rs, err = client.StartTest(ctx, rq)
	}
	if err != nil {
		return "", err
	}

	return printEntryID(cmd, rs)
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

	return printEntryID(cmd, rs)
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
	if desc := cmd.String("description"); desc != "" {
		rq.Description = openapi.PtrString(desc)
	}
	if attrs := parseAttributes(cmd.StringSlice("attr")); len(attrs) > 0 {
		rq.Attributes = attrs
	}
	if issueType := cmd.String("issue-type"); issueType != "" {
		rq.Issue = &openapi.ComEpamReportportalBaseReportingIssue{IssueType: issueType}
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
	if desc := cmd.String("description"); desc != "" {
		rq.Description = openapi.PtrString(desc)
	}
	if attrs := parseAttributes(cmd.StringSlice("attr")); len(attrs) > 0 {
		rq.Attributes = attrs
	}

	rs, err := client.FinishLaunch(ctx, cmd.String("launch-uuid"), rq)
	if err != nil {
		return err
	}

	// The UUID stays on the first line, so scripts that read it keep working.
	_, _ = fmt.Fprintln(cmd.Writer, cmd.String("launch-uuid"))
	if link := rs.GetLink(); link != "" {
		_, _ = fmt.Fprintln(cmd.Writer, link)
	}

	return nil
}
