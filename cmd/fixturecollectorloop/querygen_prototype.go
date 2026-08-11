//go:build ignore

// PROTOTYPE: estimate which fixture-collection detectors need reviewed queries.
// Run from the repository root:
//
//	go run ./cmd/fixturecollectorloop/querygen_prototype.go
//
// This is deliberately throwaway code for validating the Q24 design decision.
package main

import (
	"fmt"
	"regexp"
	"regexp/syntax"
	"sort"
	"strings"

	"github.com/ferretsecurity/deplens/internal/analyze"
)

const maxGeneratedFilenames = 16

type queryPlan struct {
	queries []string
	reason  string
}

func main() {
	rules, err := analyze.LoadDefaultRules()
	if err != nil {
		panic(err)
	}

	eligible := 0
	needsReview := 0
	for _, detector := range rules.DetectorCapabilities() {
		if contains(detector.Capabilities, "extract") {
			continue
		}
		eligible++
		plan := generateQueryPlan(detector)
		if len(plan.queries) == 0 {
			needsReview++
			fmt.Printf("NEEDS-QUERY-REVIEW  %-36s %s\n", detector.ID, plan.reason)
			continue
		}
		fmt.Printf("READY               %-36s %s\n", detector.ID, strings.Join(plan.queries, " | "))
	}

	fmt.Printf("\nEligible detectors: %d\n", eligible)
	fmt.Printf("Ready automatically: %d\n", eligible-needsReview)
	fmt.Printf("Needs query review: %d\n", needsReview)
}

func generateQueryPlan(detector analyze.DetectorCapability) queryPlan {
	var queries []string
	if detector.FilenameRegex != "" {
		filenames, ok := finiteFilenames(detector.FilenameRegex)
		if ok {
			for _, name := range filenames {
				queries = append(queries, "filename:"+quoteIfNeeded(name))
			}
		} else if extension, ok := simpleExtension(detector.FilenameRegex); ok {
			if detector.Analyzer != "" {
				return queryPlan{reason: fmt.Sprintf("extension-only search needs a content term for analyzer %q", detector.Analyzer)}
			}
			queries = append(queries, "extension:"+extension)
		} else {
			return queryPlan{reason: fmt.Sprintf("filename regex is not safely finite: %q", detector.FilenameRegex)}
		}
	}

	if detector.PathGlob != "" {
		query, ok := pathGlobQuery(detector.PathGlob)
		if !ok {
			return queryPlan{reason: fmt.Sprintf("path glob is not safely translatable: %q", detector.PathGlob)}
		}
		queries = append(queries, query)
	}

	if len(queries) == 0 {
		return queryPlan{reason: "detector has no selector"}
	}
	sort.Strings(queries)
	return queryPlan{queries: deduplicate(queries)}
}

func finiteFilenames(pattern string) ([]string, bool) {
	re, err := syntax.Parse(pattern, syntax.Perl)
	if err != nil {
		return nil, false
	}
	values, ok := enumerate(re.Simplify())
	if !ok || len(values) == 0 || len(values) > maxGeneratedFilenames {
		return nil, false
	}
	for _, value := range values {
		if value == "" || strings.ContainsAny(value, `/\\`) {
			return nil, false
		}
	}
	sort.Strings(values)
	return deduplicate(values), true
}

func enumerate(re *syntax.Regexp) ([]string, bool) {
	switch re.Op {
	case syntax.OpEmptyMatch, syntax.OpBeginLine, syntax.OpEndLine, syntax.OpBeginText, syntax.OpEndText:
		return []string{""}, true
	case syntax.OpLiteral:
		return []string{string(re.Rune)}, true
	case syntax.OpCapture:
		return enumerate(re.Sub[0])
	case syntax.OpQuest:
		values, ok := enumerate(re.Sub[0])
		if !ok {
			return nil, false
		}
		return append([]string{""}, values...), true
	case syntax.OpCharClass:
		var values []string
		for i := 0; i < len(re.Rune); i += 2 {
			for r := re.Rune[i]; r <= re.Rune[i+1]; r++ {
				values = append(values, string(r))
				if len(values) > maxGeneratedFilenames {
					return nil, false
				}
			}
		}
		return values, true
	case syntax.OpAlternate:
		var values []string
		for _, sub := range re.Sub {
			part, ok := enumerate(sub)
			if !ok {
				return nil, false
			}
			values = append(values, part...)
			if len(values) > maxGeneratedFilenames {
				return nil, false
			}
		}
		return values, true
	case syntax.OpConcat:
		values := []string{""}
		for _, sub := range re.Sub {
			part, ok := enumerate(sub)
			if !ok || len(values)*len(part) > maxGeneratedFilenames {
				return nil, false
			}
			var next []string
			for _, prefix := range values {
				for _, suffix := range part {
					next = append(next, prefix+suffix)
				}
			}
			values = next
		}
		return values, true
	default:
		return nil, false
	}
}

func simpleExtension(pattern string) (string, bool) {
	match := regexp.MustCompile(`^\^?\.\*\\\.([A-Za-z0-9_-]+)\$$`).FindStringSubmatch(pattern)
	if len(match) != 2 {
		return "", false
	}
	return match[1], true
}

func pathGlobQuery(glob string) (string, bool) {
	path := strings.TrimPrefix(glob, "**/")
	base := path[strings.LastIndex(path, "/")+1:]
	parent := strings.TrimSuffix(path, base)

	if !strings.ContainsAny(base, "*?[") {
		if strings.ContainsAny(parent, "*?[") {
			return "filename:" + quoteIfNeeded(base), true
		}
		return "path:" + quoteIfNeeded(path), true
	}
	if strings.HasPrefix(base, "*.") && !strings.ContainsAny(strings.TrimPrefix(base, "*."), "*?[") {
		extension := strings.TrimPrefix(base, "*.")
		if strings.ContainsAny(parent, "*?[") {
			return "extension:" + extension, true
		}
		return "path:" + quoteIfNeeded(strings.TrimSuffix(parent, "/")) + " extension:" + extension, true
	}
	return "", false
}

func quoteIfNeeded(value string) string {
	if strings.ContainsAny(value, " \t\"") {
		return fmt.Sprintf("%q", value)
	}
	return value
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func deduplicate(values []string) []string {
	result := values[:0]
	for _, value := range values {
		if len(result) == 0 || result[len(result)-1] != value {
			result = append(result, value)
		}
	}
	return result
}
