package analyze

import (
	"path/filepath"
	"testing"
)

func TestSwiftPackageResolvedFixturesExtractDependencies(t *testing.T) {
	tests := []struct {
		name       string
		fixtureDir string
		want       []DependencyReference
	}{
		{
			name:       "version 1 package pin",
			fixtureDir: "package-resolved-v1",
			want: []DependencyReference{{
				PackageType: "swift", Raw: "TreeSitter@0.23.2", Name: "TreeSitter", Version: "0.23.2", SourceGroup: "pins", OriginKind: OriginGit, Relationship: RelationshipInconclusive, Scope: ScopeRuntime,
				Attributes: map[string]string{"source_url": "https://example.test/tree-sitter", "source_ref": "1111111111111111111111111111111111111111", "source_ref_kind": "commit"},
			}},
		},
		{
			name:       "version 2 identity pin",
			fixtureDir: "package-resolved-v2-version",
			want: []DependencyReference{{
				PackageType: "swift", Raw: "alamofire@5.5.0", Name: "alamofire", Version: "5.5.0", SourceGroup: "pins", OriginKind: OriginGit, Relationship: RelationshipInconclusive, Scope: ScopeRuntime,
				Attributes: map[string]string{"source_url": "https://example.test/Alamofire.git", "source_ref": "2222222222222222222222222222222222222222", "source_ref_kind": "commit"},
			}},
		},
		{
			name:       "branch pin without a version",
			fixtureDir: "package-resolved-v2-branch",
			want: []DependencyReference{{
				PackageType: "swift", Raw: "llama.cpp@3333333333333333333333333333333333333333", Name: "llama.cpp", Version: "3333333333333333333333333333333333333333", SourceGroup: "pins", OriginKind: OriginGit, Relationship: RelationshipInconclusive, Scope: ScopeRuntime,
				Attributes: map[string]string{"source_url": "https://example.test/llama.cpp", "source_ref": "3333333333333333333333333333333333333333", "source_ref_kind": "commit", "source_branch": "master"},
			}},
		},
	}

	ruleset := mustLoadDefaultRules(t)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Scan(filepath.Join("..", "..", "testdata", "swift", tc.fixtureDir), nil, ruleset)
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			source := sourceForPath(t, result, "Package.resolved")
			if source.Detector != "swift-package-resolved" || source.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) {
				t.Fatalf("source = %+v", source)
			}
			if !equalDependencies(source.Dependencies, tc.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, tc.want)
			}
		})
	}
}

func TestSwiftPackageResolvedWithoutPinsIsCompleteAndEmpty(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	result, err := Scan(filepath.Join("..", "..", "testdata", "swift", "package-resolved-no-deps"), nil, ruleset)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	source := sourceForPath(t, result, "Package.resolved")
	if source.Detector != "swift-package-resolved" || source.Analysis != (SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}) || len(source.Dependencies) != 0 {
		t.Fatalf("source = %+v", source)
	}
}
