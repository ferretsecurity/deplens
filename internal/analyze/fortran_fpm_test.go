package analyze

import (
	"path/filepath"
	"testing"
)

func TestFortranFPMFixtures(t *testing.T) {
	tests := []struct {
		name     string
		fixture  string
		analysis SourceAnalysis
		want     []DependencyReference
	}{
		{
			name:     "features default profile has no dependencies",
			fixture:  "features-default-profile",
			analysis: SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete},
			want:     []DependencyReference{},
		},
		{
			name:     "runtime registry and Git dependencies",
			fixture:  "runtime-git-and-registry",
			analysis: SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete},
			want: []DependencyReference{
				{PackageType: "generic", Raw: "fortran-curl@https://github.com/interkosmos/fortran-curl.git", Name: "fortran-curl", SourceGroup: "dependencies", OriginKind: OriginGit, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://github.com/interkosmos/fortran-curl.git", "source_ref": "0.6.0", "source_ref_kind": "tag"}},
				{PackageType: "generic", Raw: "stdlib@*", Name: "stdlib", VersionConstraint: "*", SourceGroup: "dependencies", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
			},
		},
		{
			name:     "development Git dependency",
			fixture:  "dev-git-dependency",
			analysis: SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete},
			want: []DependencyReference{
				{PackageType: "generic", Raw: "test-drive@https://github.com/fortran-lang/test-drive", Name: "test-drive", SourceGroup: "dev-dependencies", OriginKind: OriginGit, Relationship: RelationshipDirect, Scope: ScopeTest, Attributes: map[string]string{"source_url": "https://github.com/fortran-lang/test-drive"}},
			},
		},
	}

	ruleset := mustLoadDefaultRules(t)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := Scan(filepath.Join("..", "..", "testdata", "fortran-fpm", test.fixture), nil, ruleset)
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			source := sourceForPath(t, result, "fpm.toml")
			if source.Detector != "fortran-fpm" || source.Analysis != test.analysis {
				t.Fatalf("source = %+v", source)
			}
			if !equalDependencies(source.Dependencies, test.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, test.want)
			}
		})
	}
}
