package analyze

import (
	"path/filepath"
	"testing"
)

func TestSoldeerLockFixturesExtractDependencies(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		want    []DependencyReference
	}{
		{
			name:    "legacy source archive",
			fixture: "lock-source",
			want: []DependencyReference{
				{PackageType: "generic", Raw: "legacy-archive@1.2.3", Name: "legacy-archive", Version: "1.2.3", SourceGroup: "dependencies", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://packages.example.test/legacy-archive-1.2.3.zip", "checksum": "legacy-checksum", "integrity": "legacy-integrity"}},
			},
		},
		{
			name:    "Git revision",
			fixture: "lock-git",
			want: []DependencyReference{
				{PackageType: "generic", Raw: "git-package@2.0.0", Name: "git-package", Version: "2.0.0", SourceGroup: "dependencies", OriginKind: OriginGit, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://git.example.test/acme/git-package.git", "source_ref": "0123456789abcdef", "source_ref_kind": "revision"}},
				{PackageType: "generic", Raw: "registry-package@4.5.6", Name: "registry-package", Version: "4.5.6", SourceGroup: "dependencies", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://packages.example.test/registry-package-4.5.6.zip", "checksum": "registry-checksum", "integrity": "registry-integrity"}},
			},
		},
		{
			name:    "modern URL archive",
			fixture: "lock-url",
			want: []DependencyReference{
				{PackageType: "generic", Raw: "modern-archive@3.4.5", Name: "modern-archive", Version: "3.4.5", SourceGroup: "dependencies", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://packages.example.test/modern-archive-3.4.5.zip", "checksum": "modern-checksum", "integrity": "modern-integrity"}},
			},
		},
	}

	ruleset := mustLoadDefaultRules(t)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := Scan(filepath.Join("..", "..", "testdata", "soldeer", test.fixture), nil, ruleset)
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			source := sourceForPath(t, result, "soldeer.lock")
			if source.Detector != "soldeer-lock" || source.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) {
				t.Fatalf("source = %+v", source)
			}
			if !equalDependencies(source.Dependencies, test.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, test.want)
			}
		})
	}
}

func TestSoldeerLockEmptyFileHasCompleteEmptyResult(t *testing.T) {
	result, err := (soldeerLockParser{}).Analyze("soldeer.lock", []byte("# no locked dependencies\n"))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if !result.Recognized || result.Analysis != (SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}) || len(result.Dependencies) != 0 {
		t.Fatalf("result = %+v", result)
	}
}
