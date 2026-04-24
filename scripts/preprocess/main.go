// Runs the yq→jq Docker pipeline that normalises the upstream ReportPortal spec into
// openapi-modified.json without requiring a POSIX shell (works on Windows, Linux, macOS).
//
// Usage: go run ./scripts/preprocess <workdir> <spec-path> <output-path>
//
//	workdir     – absolute path mounted as /workdir inside both containers
//	spec-path   – path to the input spec, relative to workdir (passed to yq inside the container)
//	output-path – absolute path where the processed JSON is written on the host
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	if len(os.Args) != 4 {
		fmt.Fprintln(os.Stderr, "usage: preprocess <workdir> <spec-path> <output-path>")
		os.Exit(2)
	}
	if err := run(os.Args[1], os.Args[2], os.Args[3]); err != nil {
		fmt.Fprintln(os.Stderr, "preprocess:", err)
		os.Exit(1)
	}
}

func run(workdir, specPath, outputPath string) (retErr error) {
	absWorkdir, err := filepath.Abs(workdir)
	if err != nil {
		return fmt.Errorf("resolve workdir: %w", err)
	}

	absOutput, err := filepath.Abs(outputPath)
	if err != nil {
		return fmt.Errorf("resolve output path: %w", err)
	}

	fmt.Fprintf(os.Stderr, "preprocess: %s → %s\n", specPath, absOutput)

	// yq: normalize YAML/JSON spec to JSON, using /workdir as the Docker working directory.
	yqCmd := exec.Command("docker", //nolint:gosec
		"run", "--rm",
		"-v", absWorkdir+":/workdir",
		"-w", "/workdir",
		"mikefarah/yq:4",
		"-o=json", ".", specPath,
	)
	yqCmd.Stderr = os.Stderr

	// jq: apply the preprocessing filter, reading JSON from yq's stdout.
	jqCmd := exec.Command("docker", //nolint:gosec
		"run", "--rm", "-i",
		"-v", absWorkdir+":/workdir",
		"ghcr.io/jqlang/jq",
		"-f", "/workdir/scripts/openapi-preprocess.jq",
	)
	jqCmd.Stderr = os.Stderr

	// Wire yq stdout → jq stdin via an in-process pipe.
	pr, pw := io.Pipe()
	yqCmd.Stdout = pw
	jqCmd.Stdin = pr

	// Write output to a temp file in the same directory as absOutput so that
	// os.Rename is guaranteed to be atomic (same filesystem). The final file is
	// only replaced once the entire pipeline succeeds; any failure leaves the
	// previous absOutput untouched.
	tmp, err := os.CreateTemp(filepath.Dir(absOutput), "preprocess-*.json.tmp")
	if err != nil {
		return fmt.Errorf("create temp output file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		_ = tmp.Close()
		if retErr != nil {
			_ = os.Remove(tmpName)
		}
	}()
	jqCmd.Stdout = tmp

	if err := yqCmd.Start(); err != nil {
		return fmt.Errorf("start yq: %w", err)
	}
	if err := jqCmd.Start(); err != nil {
		_ = pr.Close()
		_ = pw.Close()
		if yqCmd.Process != nil {
			_ = yqCmd.Process.Kill()
		}
		_ = yqCmd.Wait()
		return fmt.Errorf("start jq: %w", err)
	}

	// Close the write end of the pipe once yq exits so jq receives EOF.
	yqErrCh := make(chan error, 1)
	go func() {
		yqErr := yqCmd.Wait()
		pw.CloseWithError(yqErr)
		yqErrCh <- yqErr
	}()

	jqErr := jqCmd.Wait()
	// If jq exited early (e.g. filter error), no one drains the pipe read-end and yq
	// blocks writing into a full buffer — yqCmd.Wait() never returns. Closing pr here
	// delivers a broken-pipe error to yq so it exits promptly.
	pr.CloseWithError(jqErr)
	yqErr := <-yqErrCh

	if yqErr != nil {
		return fmt.Errorf("yq: %w", yqErr)
	}
	if jqErr != nil {
		return fmt.Errorf("jq: %w", jqErr)
	}

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp output file: %w", err)
	}

	if err := os.Rename(tmpName, absOutput); err != nil {
		return fmt.Errorf("rename temp to output: %w", err)
	}

	fmt.Fprintf(os.Stderr, "preprocess: written %s\n", absOutput)
	return nil
}
