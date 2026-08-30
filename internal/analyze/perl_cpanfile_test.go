package analyze

import (
	"path/filepath"
	"testing"
)

func TestPerlCpanfileFixturesExtractDependencies(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		want    []DependencyReference
	}{
		{
			name:    "requirements and recommendations",
			fixture: "cpanfile-recommends",
			want: []DependencyReference{
				perlCpanfileTestDependency("Registry::Codec", "2.4", "recommends", ScopeOptional),
				perlCpanfileTestDependency("Web::Client", "1.2", "requires", ScopeRuntime),
			},
		},
		{
			name:    "test and development blocks",
			fixture: "cpanfile-scopes",
			want: []DependencyReference{
				perlCpanfileTestDependency("Build::Tool", "0", "on.develop.requires", ScopeDevelopment),
				perlCpanfileTestDependency("Test::Tool", "0.01", "on.test.requires", ScopeTest),
				perlCpanfileTestDependency("Runtime::Tool", "", "requires", ScopeRuntime),
			},
		},
		{
			name:    "feature block",
			fixture: "cpanfile-features",
			want: []DependencyReference{
				perlCpanfileTestDependency("Store::SQLite", ">= 3.0", "feature.sqlite.requires", ScopeOptional),
				perlCpanfileTestDependency("Service::Core", "", "requires", ScopeRuntime),
			},
		},
	}

	ruleset := mustLoadDefaultRules(t)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Scan(filepath.Join("..", "..", "testdata", "perl", tc.fixture), nil, ruleset)
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			source := sourceForPath(t, result, "cpanfile")
			if source.Detector != "perl-cpanfile" || source.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) {
				t.Fatalf("source = %+v", source)
			}
			if !equalDependencies(source.Dependencies, tc.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, tc.want)
			}
		})
	}
}

func TestPerlCpanfileWithoutDependenciesIsAbsent(t *testing.T) {
	parser, err := newPerlCpanfileParser(perlCpanfileMatcherConfig{})
	if err != nil {
		t.Fatalf("newPerlCpanfileParser: %v", err)
	}
	result, err := parser.Analyze("cpanfile", []byte("feature 'empty' => sub {\n};\n"))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if !result.Recognized || result.Analysis != (SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}) || len(result.Dependencies) != 0 {
		t.Fatalf("result = %+v", result)
	}
}

func perlCpanfileTestDependency(name, constraint, group string, scope DependencyScope) DependencyReference {
	dependency := DependencyReference{
		PackageType:  "cpan",
		Raw:          name,
		Name:         name,
		SourceGroup:  group,
		OriginKind:   OriginRegistry,
		Relationship: RelationshipDirect,
		Scope:        scope,
	}
	if constraint != "" {
		dependency.Raw += "@" + constraint
		dependency.VersionConstraint = constraint
	}
	return dependency
}
