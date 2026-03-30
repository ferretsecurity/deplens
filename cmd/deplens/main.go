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
	cfg, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		if errors.Is(err, flag.ErrHelp) {
			return 0
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
		output = []byte(render.Human(result, ruleset.SupportedManifestTypes()))
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
	ignoreDirs []string
	rulesPath  string
}

func parseArgs(args []string) (config, error) {
	cfg := config{
		path:       ".",
		ignoreDirs: append([]string(nil), defaultIgnoreDirs...),
	}

	fs := flag.NewFlagSet("deplens", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.BoolVar(&cfg.json, "json", false, "emit machine-readable JSON output")

	var ignore string
	fs.StringVar(&ignore, "ignore", "", "comma-separated directory names to skip")
	fs.StringVar(&cfg.rulesPath, "rules", "", "path to a YAML file with manifest detection rules")

	if err := fs.Parse(args); err != nil {
		return config{}, err
	}

	if ignore != "" {
		cfg.ignoreDirs = parseIgnoreList(ignore)
	}

	if fs.NArg() > 1 {
		return config{}, fmt.Errorf("expected at most one path argument")
	}
	if fs.NArg() == 1 {
		cfg.path = fs.Arg(0)
	}

	return cfg, nil
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
