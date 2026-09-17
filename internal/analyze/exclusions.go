package analyze

import (
	"fmt"
	"path"
	"slices"
	"strings"
)

// Exclude filters a fully validated, composed ruleset, preserving detector order.
// Only explicit exclusions affect policy prerequisites; custom-base absence does not.
func (r Ruleset) Exclude(detectorIDs, checkIDs []string, presets ...string) (Ruleset, error) {
	presetIDs, err := presetExclusions(presets, r)
	if err != nil {
		return Ruleset{}, err
	}
	detectorIDs = append(slices.Clone(detectorIDs), presetIDs...)
	removed := make(map[DetectorID]bool)
	for _, id := range detectorIDs {
		if !slices.ContainsFunc(r.detectors, func(d detector) bool { return string(d.ID) == id }) {
			return Ruleset{}, fmt.Errorf("unknown detector ID %q", id)
		}
		removed[DetectorID(id)] = true
	}
	checks := make(map[CheckID]bool)
	for _, id := range checkIDs {
		if !slices.ContainsFunc(r.checks, func(c check) bool { return string(c.ID) == id }) {
			return Ruleset{}, fmt.Errorf("unknown check ID %q", id)
		}
		checks[CheckID(id)] = true
	}
	result := Ruleset{disabledDetectors: slices.Clone(r.disabledDetectors)}
	for _, d := range r.detectors {
		if removed[d.ID] {
			result.disabledDetectors = append(result.disabledDetectors, d.ID)
		} else {
			result.detectors = append(result.detectors, d)
			result.detectorIDs = append(result.detectorIDs, d.ID)
		}
	}
	for _, c := range r.checks {
		if !checks[c.ID] {
			result.checks = append(result.checks, c)
		}
	}
	return result, nil
}

// These dependencies include discovery, accepted lock alternatives, manager
// evidence, and workspace ownership. They belong to evaluators, not check IDs.
var evaluatorDetectors = map[string][]DetectorID{
	"npm-lockfile-missing":                  {"js", "js-pnpm-workspace", "js-yarnrc", "js-npm-lock", "js-npm-shrinkwrap", "js-pnpm-lock", "js-yarn"},
	"pnpm-lockfile-missing":                 {"js", "js-pnpm-workspace", "js-yarnrc", "js-npm-lock", "js-npm-shrinkwrap", "js-pnpm-lock", "js-yarn"},
	"yarn-lockfile-missing":                 {"js", "js-pnpm-workspace", "js-yarnrc", "js-npm-lock", "js-npm-shrinkwrap", "js-pnpm-lock", "js-yarn"},
	"javascript-conflicting-lockfiles":      {"js", "js-pnpm-workspace", "js-npm-lock", "js-npm-shrinkwrap", "js-pnpm-lock", "js-yarn"},
	"uv-lockfile-missing":                   {"python-pyproject", "python-uv"},
	"cargo-application-lockfile-missing":    {"rust-cargo", "rust-cargo-lock"},
	"go-sum-missing":                        {"go-mod", "go-sum"},
	"composer-application-lockfile-missing": {"php-composer", "php-composer-lock"},
	"gemfile-application-lockfile-missing":  {"ruby-gemfile", "ruby-gemfile-lock"},
}

func detectorDisabledRuns(ctx evaluationContext, sources []DependencySourceResult, configured check, disabled []DetectorID) []CheckRun {
	prerequisites := evaluatorDetectors[configured.EvaluatorType]
	removed := []string{}
	for _, id := range disabled {
		if slices.Contains(prerequisites, id) || (configured.EvaluatorType == "dependency-source-codeowners" && len(sources) == 0) {
			removed = append(removed, string(id))
		}
	}
	if len(removed) == 0 {
		return nil
	}
	slices.Sort(removed)
	roots := map[string]bool{}
	add := func(root string) { roots[projectSubject(root).ProjectRoot] = true }
	switch configured.EvaluatorType {
	case "npm-lockfile-missing", "pnpm-lockfile-missing", "yarn-lockfile-missing", "javascript-conflicting-lockfiles":
		for _, m := range ctx.javascript {
			add(m.root)
		}
		for _, w := range ctx.javascriptSpaces {
			add(w.root)
		}
	case "uv-lockfile-missing":
		for _, m := range ctx.uv {
			add(m.root)
		}
	case "cargo-application-lockfile-missing":
		for _, m := range ctx.cargo {
			add(m.root)
		}
	case "go-sum-missing":
		for _, m := range ctx.goModules {
			add(m.root)
		}
	case "composer-application-lockfile-missing":
		for _, m := range ctx.composer {
			add(m.root)
		}
	case "gemfile-application-lockfile-missing":
		for _, m := range ctx.ruby {
			add(m.root)
		}
	}
	for _, e := range ctx.parseErrors {
		if slices.Contains(prerequisites, e.detector) {
			add(path.Dir(e.path))
		}
	}
	if len(roots) == 0 {
		add(".")
	}
	runs := make([]CheckRun, 0, len(roots))
	for root := range roots {
		runs = append(runs, skippedRun(configured.ID, projectSubject(root), "detector-disabled", "prerequisite detectors disabled: "+strings.Join(removed, ", ")))
	}
	return runs
}
