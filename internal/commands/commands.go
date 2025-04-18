package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"os"

	"github.com/manifoldco/promptui"
	"github.com/urfave/cli/v3"

	"github.com/reportportal/goRP/v5/pkg/gorp"
)

type config struct {
	UUID    string   `json:"uuid"`
	Project string   `json:"project"`
	URL     *url.URL `json:"host"`
}

var (
	// RootCommand is CLI entry point
	RootCommand = []*cli.Command{
		launchCommand,
		reportCommand,
		initCommand,
	}

	initCommand = &cli.Command{
		Name:   "init",
		Usage:  "Initializes configuration cache",
		Action: initConfiguration,
	}
)

func initConfiguration(ctx context.Context, c *cli.Command) error {
	if configFilePresent() {
		prompt := promptui.Prompt{
			Label: "GoRP is already configured. Replace existing configuration?",
		}
		answer, err := prompt.Run()
		if err != nil {
			return fmt.Errorf("unable to run UI prompt: %w", err)
		}
		// do not replace. go away
		if !answerYes(answer) {
			return nil
		}
	}

	//nolint:mnd // file permission
	f, err := os.OpenFile(getConfigFile(), os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return cli.Exit(fmt.Sprintf("Cannot open config file, %s", err), 1)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			slog.Default().Error(closeErr.Error())
		}
	}()

	prompt := promptui.Prompt{
		Label: "Enter ReportPortal hostname",
	}
	hostStr, err := prompt.Run()
	if err != nil {
		return err
	}
	host, parseErr := url.Parse(hostStr)
	if parseErr != nil {
		return err
	}

	prompt = promptui.Prompt{
		Label: "UUID",
	}
	uuid, err := prompt.Run()
	if err != nil {
		return err
	}

	prompt = promptui.Prompt{
		Label: "Default Project",
	}
	project, err := prompt.Run()
	if err != nil {
		return err
	}

	err = json.NewEncoder(f).Encode(&config{
		Project: project,
		URL:     host,
		UUID:    uuid,
	})
	if err != nil {
		return cli.Exit(fmt.Sprintf("Cannot read config file. %s", err), 1)
	}

	//nolint:forbidigo //expected output
	fmt.Println("Configuration has been successfully saved!")

	return nil
}

func getConfig(c *cli.Command) (*config, error) {
	cfg := &config{}
	if configFilePresent() {
		f, err := os.Open(getConfigFile())
		if err != nil {
			return nil, err
		}
		err = json.NewDecoder(f).Decode(cfg)
		if err != nil {
			return nil, err
		}
	}
	if v := c.String("uuid"); v != "" {
		cfg.UUID = v
	}
	if v := c.String("project"); v != "" {
		cfg.Project = v
	}
	if v := c.String("host"); v != "" {
		url, parseErr := url.Parse(v)
		if parseErr != nil {
			return nil, parseErr
		}
		cfg.URL = url
	}

	if err := validateConfig(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func buildReportingClient(cmd *cli.Command) (*gorp.ReportingClient, *config, error) {
	cfg, err := getConfig(cmd)
	if err != nil {
		return nil, nil, err
	}

	return gorp.NewReportingClient(cfg.URL.String(), cfg.Project, cfg.UUID), cfg, nil
}

func buildClient(cmd *cli.Command) (*gorp.Client, *config, error) {
	cfg, err := getConfig(cmd)
	if err != nil {
		return nil, nil, err
	}

	return gorp.NewClient(cfg.URL, cfg.UUID), cfg, nil
}
