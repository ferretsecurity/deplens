// fixturecollectorloop is a repository-development tool for collecting
// reviewable, provenance-backed dependency-source examples. It is intentionally
// separate from the public deplens scanner CLI.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ferretsecurity/deplens/internal/analyze"
	"github.com/ferretsecurity/deplens/internal/fixturecollector"
)

const defaultProgressPath = "testdata/corpus/collection-progress.yaml"

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	return runWithInventory(args, stdout, stderr, analyze.DefaultDetectorInventory, fixturecollector.NewProgress)
}

func runWithInventory(args []string, stdout, stderr io.Writer, inventorySource func() ([]analyze.DetectorInventoryEntry, error), progressBuilder func([]fixturecollector.Detector) (fixturecollector.Progress, error)) int {
	if len(args) == 0 {
		printUsage(stderr)
		return 1
	}
	switch args[0] {
	case "initialize-progress":
		return initializeProgress(args[1:], stdout, stderr, inventorySource, progressBuilder)
	case "run":
		fmt.Fprintln(stderr, "error: collection execution is not yet available")
		return 1
	case "-h", "--help", "help":
		printUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "error: unknown operation %q\n", args[0])
		printUsage(stderr)
		return 1
	}
}

func initializeProgress(args []string, stdout, stderr io.Writer, inventorySource func() ([]analyze.DetectorInventoryEntry, error), progressBuilder func([]fixturecollector.Detector) (fixturecollector.Progress, error)) int {
	fs := flag.NewFlagSet("initialize-progress", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	root := fs.String("root", ".", "repository root")
	progressPath := fs.String("progress", defaultProgressPath, "progress document path, relative to root unless absolute")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printUsage(stdout)
			return 0
		}
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "error: initialize-progress accepts no positional arguments")
		return 1
	}

	inventory, err := inventorySource()
	if err != nil {
		fmt.Fprintf(stderr, "error: load detector inventory: %v\n", err)
		return 1
	}
	progress, err := progressBuilder(convertInventory(inventory))
	if err != nil {
		fmt.Fprintf(stderr, "error: initialize progress: %v\n", err)
		return 1
	}
	if err := progress.ValidateInventory(convertInventory(inventory)); err != nil {
		fmt.Fprintf(stderr, "error: validate detector coverage: %v\n", err)
		return 1
	}
	data, err := progress.MarshalYAML()
	if err != nil {
		fmt.Fprintf(stderr, "error: encode progress: %v\n", err)
		return 1
	}
	path := *progressPath
	if !filepath.IsAbs(path) {
		path = filepath.Join(*root, path)
	}
	if _, err := os.Stat(path); err == nil {
		fmt.Fprintf(stderr, "error: progress document already exists: %s\n", path)
		return 1
	} else if !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintf(stderr, "error: inspect progress document: %v\n", err)
		return 1
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fmt.Fprintf(stderr, "error: create progress directory: %v\n", err)
		return 1
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		fmt.Fprintf(stderr, "error: write progress document: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "initialized %s with %d eligible detectors\n", path, len(progress.Detectors))
	return 0
}

func convertInventory(entries []analyze.DetectorInventoryEntry) []fixturecollector.Detector {
	result := make([]fixturecollector.Detector, len(entries))
	for i, entry := range entries {
		result[i] = fixturecollector.Detector{ID: entry.ID, Form: entry.Form, Roles: entry.Roles, FilenameRegex: entry.FilenameRegex, PathGlob: entry.PathGlob, Analyzer: entry.Analyzer, AnalyzerConfig: entry.AnalyzerConfig, Capabilities: entry.Capabilities}
	}
	return result
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, "Usage: fixturecollectorloop <operation> [flags]\n\nOperations:\n  initialize-progress  create the reviewed collection progress document\n  run                  execute reviewed collection iterations\n")
}
