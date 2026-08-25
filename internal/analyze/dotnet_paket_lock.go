package analyze

import "strings"

type dotnetPaketLockParser struct{}

func newDotnetPaketLockParser(dotnetPaketLockMatcherConfig) (sourceAnalyzer, error) {
	return dotnetPaketLockParser{}, nil
}

func (dotnetPaketLockParser) Analyze(_ string, content []byte) (sourceAnalyzerResult, error) {
	dependencies := make([]DependencyReference, 0)
	group := "default"
	section := ""
	remote := ""
	recognized := false

	for _, line := range strings.Split(strings.ReplaceAll(string(content), "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		if name, ok := paketLockGroup(trimmed); ok {
			group = name
			section = ""
			remote = ""
			continue
		}

		switch strings.ToUpper(trimmed) {
		case "NUGET", "GITHUB":
			section = strings.ToUpper(trimmed)
			remote = ""
			recognized = true
			continue
		}

		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		if indent == 2 && strings.HasPrefix(strings.ToLower(trimmed), "remote:") {
			remote = strings.TrimSpace(trimmed[len("remote:"):])
			continue
		}
		if indent != 4 {
			continue
		}

		name, version, ok := paketLockResolvedEntry(trimmed)
		if !ok {
			continue
		}
		switch section {
		case "NUGET":
			dependencies = append(dependencies, paketLockNuGetDependency(name, version, group, remote))
		case "GITHUB":
			if remote != "" {
				dependencies = append(dependencies, paketLockGitHubDependency(name, version, group, remote))
			}
		}
	}

	if !recognized {
		return sourceAnalyzerResult{}, nil
	}
	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, nil), nil
}

func paketLockGroup(line string) (string, bool) {
	fields := strings.Fields(line)
	if len(fields) != 2 || !strings.EqualFold(fields[0], "GROUP") || fields[1] == "" {
		return "", false
	}
	return fields[1], true
}

func paketLockResolvedEntry(line string) (name, version string, ok bool) {
	before, after, found := strings.Cut(line, " (")
	if !found || before == "" {
		return "", "", false
	}
	version, _, found = strings.Cut(after, ")")
	if !found || version == "" {
		return "", "", false
	}
	return before, version, true
}

func paketLockNuGetDependency(name, version, group, remote string) DependencyReference {
	dependency := DependencyReference{
		PackageType:  "nuget",
		Raw:          name + "@" + version,
		Name:         name,
		Version:      version,
		SourceGroup:  group,
		OriginKind:   OriginRegistry,
		Relationship: RelationshipInconclusive,
		Scope:        paketGroupScope(group),
	}
	if remote != "" {
		dependency.Attributes = map[string]string{"source_url": remote}
	}
	return dependency
}

func paketLockGitHubDependency(module, commit, group, remote string) DependencyReference {
	repository := strings.TrimSuffix(remote, ".git")
	sourceURL := repository
	if !strings.HasPrefix(sourceURL, "http://") && !strings.HasPrefix(sourceURL, "https://") {
		sourceURL = "https://github.com/" + sourceURL
	}
	if !strings.HasSuffix(sourceURL, ".git") {
		sourceURL += ".git"
	}
	name := repository + "/" + module
	return DependencyReference{
		PackageType:  "generic",
		Raw:          name + "@" + commit,
		Name:         name,
		Version:      commit,
		SourceGroup:  group,
		OriginKind:   OriginGit,
		Relationship: RelationshipInconclusive,
		Scope:        paketGroupScope(group),
		Attributes: map[string]string{
			"source_url":      sourceURL,
			"source_path":     module,
			"source_ref":      commit,
			"source_ref_kind": "commit",
		},
	}
}
