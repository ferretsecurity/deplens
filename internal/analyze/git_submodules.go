package analyze

import (
	"fmt"
	"strings"

	ini "gopkg.in/ini.v1"
)

type gitSubmodulesMatcherConfig struct{}

type gitSubmodulesParser struct{}

func newGitSubmodulesParser(gitSubmodulesMatcherConfig) (sourceAnalyzer, error) {
	return gitSubmodulesParser{}, nil
}

func (gitSubmodulesParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	file, err := ini.LoadSources(ini.LoadOptions{
		AllowNestedValues:        true,
		SpaceBeforeInlineComment: true,
	}, content)
	if err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse Git submodules file %q: %w", path, err)
	}

	dependencies := make([]DependencyReference, 0)
	recognized := false
	for _, section := range file.Sections() {
		name, ok := gitSubmoduleName(section.Name())
		if !ok {
			continue
		}
		recognized = true

		url := gitSubmoduleValue(section, "url")
		if url == "" {
			continue
		}

		dependency := DependencyReference{
			Raw:          name + "@" + url,
			Name:         name,
			SourceGroup:  "submodules",
			OriginKind:   OriginGit,
			Relationship: RelationshipDirect,
			Scope:        ScopeRuntime,
			Attributes:   map[string]string{"source_url": url},
		}
		if sourcePath := gitSubmoduleValue(section, "path"); sourcePath != "" {
			dependency.Attributes["source_path"] = sourcePath
		}
		if branch := gitSubmoduleValue(section, "branch"); branch != "" {
			dependency.Attributes["source_ref"] = branch
			dependency.Attributes["source_ref_kind"] = "branch"
		}
		dependencies = append(dependencies, dependency)
	}
	if !recognized {
		return sourceAnalyzerResult{}, nil
	}

	sortDependencyReferences(dependencies)
	return sourceAnalyzerResult{
		Recognized:   true,
		Analysis:     completeAnalysis(dependencies),
		Dependencies: dependencies,
	}, nil
}

func gitSubmoduleValue(section *ini.Section, name string) string {
	for _, key := range section.Keys() {
		if strings.EqualFold(key.Name(), name) {
			return strings.TrimSpace(key.String())
		}
	}
	return ""
}

func gitSubmoduleName(section string) (string, bool) {
	section = strings.TrimSpace(section)
	separator := strings.IndexAny(section, " \t")
	if separator < 0 || !strings.EqualFold(section[:separator], "submodule") {
		return "", false
	}

	quotedName := strings.TrimSpace(section[separator:])
	if len(quotedName) < 2 || quotedName[0] != '"' || quotedName[len(quotedName)-1] != '"' {
		return "", false
	}
	name := strings.TrimSpace(quotedName[1 : len(quotedName)-1])
	return name, name != ""
}
