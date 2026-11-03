package analyze

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
)

type pythonPyprojectParserConfig struct{}

type pythonPyprojectParser struct {
	static tomlQueryParser
}

func newPythonPyprojectParser(pythonPyprojectParserConfig) (sourceAnalyzer, error) {
	queries := make([]tomlQuery, 0, 6)
	for _, path := range []string{
		"build-system.requires[]",
		"project.dependencies[]",
		"project.optional-dependencies.*[]",
		"dependency-groups.*[]",
		"tool.poetry.dependencies",
		"tool.poetry.group.*.dependencies",
	} {
		query, err := compileTOMLQuery(path)
		if err != nil {
			return nil, err
		}
		queries = append(queries, query)
	}
	return pythonPyprojectParser{static: tomlQueryParser{queries: queries}}, nil
}

func (p pythonPyprojectParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	return p.AnalyzeInRoot(filepath.Dir(path), path, content)
}

func (p pythonPyprojectParser) AnalyzeInRoot(scanRoot, path string, content []byte) (sourceAnalyzerResult, error) {
	var document map[string]any
	if err := toml.Unmarshal(content, &document); err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse toml file %q: %w", path, err)
	}
	result := p.static.analyze(document)
	dynamic := pyprojectValue(document, "project", "dynamic")
	if dynamic == nil {
		return result, nil
	}
	fields, ok := dynamic.([]any)
	if !ok {
		return pythonExtractionResult(result.Dependencies, []string{"project.dynamic must be a list of field names"}, "python-pyproject-incomplete"), nil
	}
	declared := make(map[string]bool)
	var incomplete []string
	for _, field := range fields {
		name, ok := field.(string)
		if !ok {
			incomplete = append(incomplete, "project.dynamic must contain only field names")
			continue
		}
		declared[name] = true
	}
	for _, field := range []string{"dependencies", "optional-dependencies"} {
		if !declared[field] {
			continue
		}
		if pyprojectValue(document, "project", field) != nil {
			incomplete = append(incomplete, "project."+field+" is declared both statically and dynamically")
		}
		directive := pyprojectValue(document, "tool", "setuptools", "dynamic", field)
		if field == "dependencies" {
			dependencies, messages := readSetuptoolsDependencyFiles(scanRoot, path, directive, "project.dependencies")
			result.Dependencies = append(result.Dependencies, dependencies...)
			incomplete = append(incomplete, messages...)
			continue
		}
		extras, ok := directive.(map[string]any)
		if !ok {
			incomplete = append(incomplete, "project.optional-dependencies is dynamic without readable setuptools file declarations")
			continue
		}
		names := make([]string, 0, len(extras))
		for name := range extras {
			names = append(names, name)
		}
		slices.Sort(names)
		for _, name := range names {
			dependencies, messages := readSetuptoolsDependencyFiles(scanRoot, path, extras[name], "project.optional-dependencies."+name)
			result.Dependencies = append(result.Dependencies, dependencies...)
			incomplete = append(incomplete, messages...)
		}
	}
	if declared["dependencies"] || declared["optional-dependencies"] || len(incomplete) > 0 {
		return pythonExtractionResult(result.Dependencies, incomplete, "python-pyproject-incomplete"), nil
	}
	return result, nil
}

func pyprojectValue(document map[string]any, path ...string) any {
	var value any = document
	for _, key := range path {
		table, ok := value.(map[string]any)
		if !ok {
			return nil
		}
		value = table[key]
	}
	return value
}

func readSetuptoolsDependencyFiles(scanRoot, manifest string, directive any, group string) ([]DependencyReference, []string) {
	table, ok := directive.(map[string]any)
	if !ok || len(table) != 1 {
		return nil, []string{group + " is dynamic without a supported setuptools file declaration"}
	}
	var files []string
	switch value := table["file"].(type) {
	case string:
		files = append(files, value)
	case []any:
		for _, item := range value {
			file, ok := item.(string)
			if !ok {
				return nil, []string{group + " setuptools file list must contain only paths"}
			}
			files = append(files, file)
		}
	}
	if len(files) == 0 {
		return nil, []string{group + " requires a non-empty setuptools file declaration"}
	}
	root, err := os.OpenRoot(scanRoot)
	if err != nil {
		return nil, []string{fmt.Sprintf("%s cannot open scan root: %v", group, err)}
	}
	defer root.Close()
	manifestDir, err := filepath.Rel(scanRoot, filepath.Dir(manifest))
	if err != nil {
		return nil, []string{fmt.Sprintf("%s cannot locate manifest in scan root: %v", group, err)}
	}
	var dependencies []DependencyReference
	var incomplete []string
	for _, file := range files {
		if strings.TrimSpace(file) == "" || filepath.IsAbs(file) {
			incomplete = append(incomplete, fmt.Sprintf("%s has invalid relative file path %q", group, file))
			continue
		}
		relative := manifestDir + string(filepath.Separator) + file
		content, err := readSetuptoolsFile(root, relative)
		if err != nil {
			incomplete = append(incomplete, fmt.Sprintf("%s cannot read setuptools file %q within scan root: %v", group, file, err))
			continue
		}
		for lineNumber, line := range strings.Split(string(content), "\n") {
			raw := strings.TrimSpace(stripPythonLineComment(line))
			if raw == "" {
				continue
			}
			dependency, err := parsePythonRequirement(raw)
			if err != nil {
				incomplete = append(incomplete, fmt.Sprintf("%s file %q line %d: %v", group, file, lineNumber+1, err))
				continue
			}
			dependency.SourceGroup = group
			dependencies = append(dependencies, dependency)
		}
	}
	return dependencies, incomplete
}

func readSetuptoolsFile(root *os.Root, path string) ([]byte, error) {
	info, err := root.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("not a regular file")
	}
	return root.ReadFile(path)
}
