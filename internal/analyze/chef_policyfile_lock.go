package analyze

import (
	"encoding/json"
	"fmt"
	"strings"
)

type chefPolicyfileLockParser struct{}

type chefPolicyfileLockFile struct {
	RunList       []string                          `json:"run_list"`
	CookbookLocks map[string]chefPolicyfileCookbook `json:"cookbook_locks"`
}

type chefPolicyfileCookbook struct {
	Version       string                      `json:"version"`
	Origin        string                      `json:"origin"`
	SourceOptions chefPolicyfileSourceOptions `json:"source_options"`
	SCMInfo       chefPolicyfileSCMInfo       `json:"scm_info"`
}

type chefPolicyfileSourceOptions struct {
	Path           string `json:"path"`
	ArtifactServer string `json:"artifactserver"`
}

type chefPolicyfileSCMInfo struct {
	SCM      string `json:"scm"`
	Remote   string `json:"remote"`
	Revision string `json:"revision"`
}

func newChefPolicyfileLockParser(chefPolicyfileLockMatcherConfig) (sourceAnalyzer, error) {
	return chefPolicyfileLockParser{}, nil
}

func (chefPolicyfileLockParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	var lock chefPolicyfileLockFile
	if err := json.Unmarshal(content, &lock); err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse Policyfile.lock.json %q: %w", path, err)
	}
	if lock.CookbookLocks == nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse Policyfile.lock.json %q: missing cookbook_locks", path)
	}

	directCookbooks := chefPolicyfileLockDirectCookbooks(lock.RunList)
	dependencies := make([]DependencyReference, 0, len(lock.CookbookLocks))
	incomplete := make([]string, 0)
	for name, cookbook := range lock.CookbookLocks {
		if name == "" || cookbook.Version == "" {
			incomplete = append(incomplete, fmt.Sprintf("Policyfile.lock.json cookbook %q has no resolved version", name))
			continue
		}
		dependencies = append(dependencies, chefPolicyfileLockDependency(name, cookbook, directCookbooks))
	}

	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func chefPolicyfileLockDirectCookbooks(runList []string) map[string]struct{} {
	directCookbooks := make(map[string]struct{})
	for _, entry := range runList {
		if !strings.HasPrefix(entry, "recipe[") || !strings.HasSuffix(entry, "]") {
			continue
		}
		cookbook := strings.TrimSuffix(strings.TrimPrefix(entry, "recipe["), "]")
		if separator := strings.Index(cookbook, "::"); separator >= 0 {
			cookbook = cookbook[:separator]
		}
		if cookbook != "" {
			directCookbooks[cookbook] = struct{}{}
		}
	}
	return directCookbooks
}

func chefPolicyfileLockDependency(name string, cookbook chefPolicyfileCookbook, directCookbooks map[string]struct{}) DependencyReference {
	dependency := DependencyReference{
		Raw:          name + "@" + cookbook.Version,
		Name:         name,
		Version:      cookbook.Version,
		SourceGroup:  "default",
		OriginKind:   OriginRegistry,
		Relationship: RelationshipTransitive,
		Scope:        ScopeRuntime,
	}
	if _, direct := directCookbooks[name]; direct {
		dependency.Relationship = RelationshipDirect
	}

	attributes := make(map[string]string)
	switch {
	case cookbook.SourceOptions.Path != "":
		dependency.OriginKind = OriginPath
		attributes["source_path"] = cookbook.SourceOptions.Path
		if cookbook.SCMInfo.Revision != "" {
			attributes["source_ref"] = cookbook.SCMInfo.Revision
			attributes["source_ref_kind"] = "revision"
		}
	case cookbook.SCMInfo.Remote != "":
		dependency.OriginKind = OriginGit
		attributes["source_url"] = cookbook.SCMInfo.Remote
		if cookbook.SCMInfo.Revision != "" {
			attributes["source_ref"] = cookbook.SCMInfo.Revision
			attributes["source_ref_kind"] = "revision"
		}
	case cookbook.Origin != "":
		attributes["source_url"] = cookbook.Origin
	case cookbook.SourceOptions.ArtifactServer != "":
		attributes["source_url"] = cookbook.SourceOptions.ArtifactServer
	}
	if len(attributes) > 0 {
		dependency.Attributes = attributes
	}
	return dependency
}
