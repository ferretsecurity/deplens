package analyze

import (
	"path/filepath"
	"testing"
)

func TestPerlMakefilePLFixtures(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		want    []DependencyReference
	}{
		{
			name:    "Module Install prerequisites",
			fixture: "makefile-pl-module-install",
			want: []DependencyReference{
				perlMakefilePLTestDependency("HTML::Lint", "0", "build_requires", ScopeBuild),
				perlMakefilePLTestDependency("Test::HTML::Lint", "2.06", "build_requires", ScopeBuild),
				perlMakefilePLTestDependency("Test::More", "0.42", "build_requires", ScopeBuild),
				perlMakefilePLTestDependency("B::Hooks::EndOfScope", "0.05", "requires", ScopeRuntime),
				perlMakefilePLTestDependency("Devel::Declare", "0", "requires", ScopeRuntime),
				perlMakefilePLTestDependency("Exporter::Lite", "0", "requires", ScopeRuntime),
				perlMakefilePLTestDependency("HTML::Entities", "0", "requires", ScopeRuntime),
				perlMakefilePLTestDependency("String::BufferStack", "1.15", "requires", ScopeRuntime),
				perlMakefilePLTestDependency("Sub::Install", "0", "requires", ScopeRuntime),
			},
		},
		{
			name:    "ExtUtils empty prerequisite map",
			fixture: "makefile-pl-empty-prereq",
			want:    []DependencyReference{},
		},
		{
			name:    "ExtUtils without prerequisites",
			fixture: "makefile-pl-no-prereq",
			want:    []DependencyReference{},
		},
	}

	ruleset := mustLoadDefaultRules(t)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Scan(filepath.Join("..", "..", "testdata", "perl", tc.fixture), nil, ruleset)
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			source := sourceForPath(t, result, "Makefile.PL")
			if source.Detector != "perl-makefile-pl" {
				t.Fatalf("detector = %q, want perl-makefile-pl", source.Detector)
			}
			wantAnalysis := SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}
			if len(tc.want) > 0 {
				wantAnalysis = SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}
			}
			if source.Analysis != wantAnalysis {
				t.Fatalf("analysis = %+v, want %+v", source.Analysis, wantAnalysis)
			}
			if !equalDependencies(source.Dependencies, tc.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, tc.want)
			}
		})
	}
}

func perlMakefilePLTestDependency(name, constraint, group string, scope DependencyScope) DependencyReference {
	return DependencyReference{
		PackageType:       "cpan",
		Raw:               name + "@" + constraint,
		Name:              name,
		VersionConstraint: constraint,
		SourceGroup:       group,
		OriginKind:        OriginRegistry,
		Relationship:      RelationshipDirect,
		Scope:             scope,
	}
}
