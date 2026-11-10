package commands

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v3"

	"github.com/reportportal/goRP/v5/pkg/openapi"
)

var (
	errHostNotSet    = errors.New("host is not set")
	errProjectNotSet = errors.New("project is not set")
	errApiKeyNotSet  = errors.New("api key is not set")
	errNoEntryID     = errors.New("response does not contain an entry id")
)

// entryID returns the ID of a created entry or an error if the server did not return one.
func entryID(rs *openapi.EntryCreatedAsyncRS) (string, error) {
	if id := rs.GetId(); id != "" {
		return id, nil
	}

	return "", errNoEntryID
}

// printEntryID writes the ID of a created entry to cmd.Writer and returns it.
func printEntryID(cmd *cli.Command, rs *openapi.EntryCreatedAsyncRS) (string, error) {
	id, err := entryID(rs)
	if err != nil {
		return "", err
	}
	_, _ = fmt.Fprintln(cmd.Writer, id)

	return id, nil
}

func checkStdinEmpty() error {
	fi, statErr := os.Stdin.Stat()
	if statErr != nil {
		return cli.Exit(fmt.Errorf("failed to get stdin stat: %w", statErr), 1)
	}
	if fi.Mode()&os.ModeCharDevice != 0 {
		// os.Stdin is attached to a terminal without redirection.
		// Pipes report size 0 until the writer produces output, so size can't be used here.
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
