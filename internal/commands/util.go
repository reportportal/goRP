package commands

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v3"
)

var (
	errHostNotSet    = errors.New("host is not set")
	errProjectNotSet = errors.New("project is not set")
	errApiKeyNotSet  = errors.New("api key is not set")
)

func checkStdinEmpty() error {
	fi, statErr := os.Stdin.Stat()
	if statErr != nil {
		return cli.Exit(fmt.Errorf("failed to get stdin stat: %w", statErr), 1)
	}
	if fi.Mode()&os.ModeCharDevice != 0 || fi.Size() == 0 {
		// os.Stdin is empty (or attached to a terminal without redirection)
		return cli.Exit("No input detected on stdin. Exiting...", 1)
	}
	return nil
}

func validateConfig(cfg *clientConfig) error {
	if cfg.ApiKey == "" {
		return errApiKeyNotSet
	}

	if cfg.Project == "" {
		return errProjectNotSet
	}

	if cfg.URL == "" {
		return errHostNotSet
	}

	return nil
}

func answerYes(answer string) bool {
	lower := strings.ToLower(answer)

	return lower == "y" || lower == "yes"
}

func configFilePresent() bool {
	_, err := os.Stat(getConfigFile())

	return !os.IsNotExist(err)
}

func getConfigFile() string {
	return filepath.Join(getHomeDir(), ".gorp")
}

func getHomeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	curUser, err := user.Current()
	if err != nil {
		// well, sheesh
		return "."
	}

	return curUser.HomeDir
}
