package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ferretsecurity/deplens/internal/analyze"
	"github.com/ferretsecurity/deplens/internal/render"
)

var defaultIgnoreDirs = []string{
	".git",
	"node_modules",
	".venv",
	"venv",
	"vendor",
	".tox",
	".mypy_cache",
	".pytest_cache",
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	cfg, usage, err := parseArgs(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			if _, writeErr := stdout.Write([]byte(usage)); writeErr != nil {
				fmt.Fprintf(stderr, "error: writing output: %v\n", writeErr)
				return 1
			}
			return 0
		}
		fmt.Fprintf(stderr, "error: %v\n", err)
		if usage != "" {
			fmt.Fprintln(stderr)
			if _, writeErr := stderr.Write([]byte(usage)); writeErr != nil {
				fmt.Fprintf(stderr, "error: writing output: %v\n", writeErr)
				return 1
			}
		}
		return 1
	}

	ruleset, err := loadRuleset(cfg.rulesPath)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	result, err := analyze.Scan(cfg.path, cfg.ignoreDirs, ruleset)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	var output []byte
	if cfg.json {
		output, err = render.JSON(result)
	} else {
		output = []byte(render.Human(result, ruleset.SupportedManifestTypes(), render.HumanOptions{
			ShowEmpty: cfg.showEmpty,
		}))
	}
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	if _, err := stdout.Write(output); err != nil {
		fmt.Fprintf(stderr, "error: writing output: %v\n", err)
		return 1
	}
	if len(output) == 0 || output[len(output)-1] != '\n' {
		fmt.Fprintln(stdout)
	}

	return 0
}

type config struct {
	path       string
	json       bool
	showEmpty  bool
	ignoreDirs []string
	rulesPath  string
}

func parseArgs(args []string) (config, string, error) {
	cfg := config{
		path:       ".",
		ignoreDirs: append([]string(nil), defaultIgnoreDirs...),
	}

	fs := flag.NewFlagSet("deplens", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {
	}
	renderUsage := func() string {
		var usage strings.Builder
		fmt.Fprintln(&usage, "Usage: deplens [flags] [path]")
		fmt.Fprintln(&usage)
		fmt.Fprintln(&usage, "Scan a directory tree and report dependency-related manifests.")
		fmt.Fprintln(&usage)
		fmt.Fprintln(&usage, "Flags:")
		fs.SetOutput(&usage)
		fs.PrintDefaults()
		fs.SetOutput(io.Discard)
		return usage.String()
	}
	fs.BoolVar(&cfg.json, "json", false, "emit machine-readable JSON output")
	fs.BoolVar(&cfg.showEmpty, "show-empty", false, "include matched manifests that were confirmed to have no dependencies")

	var ignore string
	fs.StringVar(&ignore, "ignore", "", "comma-separated directory names to skip")
	fs.StringVar(&cfg.rulesPath, "rules", "", "path to a YAML file with manifest detection rules")

	if err := fs.Parse(args); err != nil {
		return config{}, renderUsage(), err
	}

	if ignore != "" {
		cfg.ignoreDirs = parseIgnoreList(ignore)
	}

	if fs.NArg() > 1 {
		return config{}, renderUsage(), fmt.Errorf("expected at most one path argument")
	}
	if fs.NArg() == 1 {
		cfg.path = fs.Arg(0)
	}

	return cfg, renderUsage(), nil
}

func parseIgnoreList(value string) []string {
	parts := strings.Split(value, ",")
	ignoreDirs := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		ignoreDirs = append(ignoreDirs, part)
	}
	return ignoreDirs
}

func loadRuleset(path string) (analyze.Ruleset, error) {
	if path == "" {
		return analyze.LoadDefaultRules()
	}
	return analyze.LoadRulesFile(path)
}
