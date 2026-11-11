package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// DiscoveredSuite represents a Ginkgo test suite found in the filesystem.
type DiscoveredSuite struct {
	Directory    string // Absolute path to the suite directory
	RelativePath string // Relative to project root
	Name         string // From RunSpecs(t, "Suite Name")
	FilePath     string // Path to *_suite_test.go file
}

var runSpecsPattern = regexp.MustCompile(`RunSpecs\s*\([^,]+,\s*"([^"]+)"`)

// validateAndCleanPath sanitizes a user-provided path to prevent path traversal.
func validateAndCleanPath(userPath, workingDir string) (string, error) {
	if userPath == "" {
		return "", errors.New("empty path provided")
	}
	cleanedPath := filepath.Clean(userPath)
	var absPath string
	if filepath.IsAbs(cleanedPath) {
		absPath = cleanedPath
	} else {
		absPath = filepath.Join(workingDir, cleanedPath)
	}
	relPath, err := filepath.Rel(workingDir, absPath)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}
	if strings.HasPrefix(relPath, ".."+string(filepath.Separator)) || relPath == ".." {
		return "", fmt.Errorf("path traversal detected: %s escapes project root", userPath)
	}
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("path does not exist: %s", absPath)
		}
		return "", fmt.Errorf("cannot access path: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("path is not a directory: %s", absPath)
	}
	return absPath, nil
}

// validateFilePath ensures filePath is within allowedDir and is a regular file.
func validateFilePath(filePath, allowedDir string) error {
	cleanedFile := filepath.Clean(filePath)
	cleanedDir := filepath.Clean(allowedDir)
	relPath, err := filepath.Rel(cleanedDir, cleanedFile)
	if err != nil {
		return fmt.Errorf("invalid file path: %w", err)
	}
	if strings.HasPrefix(relPath, ".."+string(filepath.Separator)) || relPath == ".." {
		return fmt.Errorf("path traversal detected: %s is outside %s", filePath, allowedDir)
	}
	info, err := os.Stat(cleanedFile)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file does not exist: %s", cleanedFile)
		}
		return fmt.Errorf("cannot access file: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("path is a directory, not a file: %s", cleanedFile)
	}
	return nil
}

// discoverSuites finds all Ginkgo test suites under targetPath.
// When recursive is false only the target directory itself is checked.
func discoverSuites(targetPath string, recursive bool) ([]DiscoveredSuite, error) {
	projectRoot, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get working directory: %w", err)
	}
	absTargetPath, err := validateAndCleanPath(targetPath, projectRoot)
	if err != nil {
		return nil, fmt.Errorf("invalid target path: %w", err)
	}

	var suites []DiscoveredSuite

	if recursive {
		err = filepath.Walk(absTargetPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				return nil
			}
			base := filepath.Base(path)
			// Mirror Ginkgo's own directory skip rules.
			if base == "vendor" || strings.HasPrefix(base, ".") || strings.HasPrefix(base, "_") {
				return filepath.SkipDir
			}
			suiteFile, ferr := findSuiteTestFile(path)
			if ferr != nil || suiteFile == "" {
				return nil
			}
			suiteName, _ := parseSuiteNameFromFile(suiteFile)
			if suiteName == "" {
				suiteName = filepath.Base(path)
			}
			relPath, _ := filepath.Rel(projectRoot, path)
			suites = append(suites, DiscoveredSuite{
				Directory:    path,
				RelativePath: relPath,
				Name:         suiteName,
				FilePath:     suiteFile,
			})
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("failed to walk directory tree: %w", err)
		}
	} else {
		suiteFile, err := findSuiteTestFile(absTargetPath)
		if err != nil {
			return nil, fmt.Errorf("failed to find suite file in %s: %w", absTargetPath, err)
		}
		if suiteFile == "" {
			return nil, fmt.Errorf(
				"no *_suite_test.go file found in %s\n  Hint: use -r to scan subdirectories, or pass an explicit suite path (e.g. ginkgo-rp calculator/)",
				absTargetPath,
			)
		}
		suiteName, _ := parseSuiteNameFromFile(suiteFile)
		if suiteName == "" {
			suiteName = filepath.Base(absTargetPath)
		}
		relPath, _ := filepath.Rel(projectRoot, absTargetPath)
		suites = append(suites, DiscoveredSuite{
			Directory:    absTargetPath,
			RelativePath: relPath,
			Name:         suiteName,
			FilePath:     suiteFile,
		})
	}
	return suites, nil
}

// findSuiteTestFile returns the path to the *_suite_test.go file in directory,
// or empty string if none is found.
func findSuiteTestFile(directory string) (string, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return "", fmt.Errorf("failed to read directory %s: %w", directory, err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, "_suite_test.go") {
			return filepath.Join(directory, name), nil
		}
	}
	return "", nil
}

// parseSuiteNameFromFile extracts the suite name from RunSpecs(t, "Suite Name").
func parseSuiteNameFromFile(filePath string) (string, error) {
	if err := validateFilePath(filePath, filepath.Dir(filePath)); err != nil {
		return "", fmt.Errorf("file validation failed: %w", err)
	}
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", filePath, err)
	}
	matches := runSpecsPattern.FindStringSubmatch(string(content))
	if len(matches) < 2 {
		return "", fmt.Errorf("could not find RunSpecs pattern in %s", filePath)
	}
	suiteName := strings.TrimSpace(matches[1])
	if suiteName == "" {
		return "", fmt.Errorf("empty suite name in %s", filePath)
	}
	return suiteName, nil
}
