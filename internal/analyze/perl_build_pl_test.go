package analyze

import (
	"path/filepath"
	"testing"
)

func TestPerlBuildPLFixturesExtractDependencies(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		want    []DependencyReference
	}{
		{
			name:    "all prerequisite groups",
			fixture: "build-pl-prerequisite-groups",
			want: []DependencyReference{
				perlBuildPLTestDependency("Builder::Tool", "0.3601", "build_requires", ScopeBuild),
				perlBuildPLTestDependency("Module::Build", "0.3601", "configure_requires", ScopeBuild),
				perlBuildPLTestDependency("Unicode::ICU::Collator", "0", "recommends", ScopeOptional),
				perlBuildPLTestDependency("Runtime::Core", "1.2", "requires", ScopeRuntime),
				perlBuildPLTestDependency("Test::More", "0.88", "test_requires", ScopeTest),
			},
		},
		{
			name:    "named argument hash",
			fixture: "build-pl-named-arguments",
			want: []DependencyReference{
				perlBuildPLTestDependency("Module::Build", "0.28", "build_requires", ScopeBuild),
				perlBuildPLTestDependency("Module::Build", "0.28", "configure_requires", ScopeBuild),
				perlBuildPLTestDependency("perl", "5.008005", "requires", ScopeRuntime),
				perlBuildPLTestDependency("Test::More", "0", "test_requires", ScopeTest),
			},
		},
		{
			name:    "subclass constructor",
			fixture: "build-pl-subclass-constructor",
			want: []DependencyReference{
				perlBuildPLTestDependency("Module::Build", "0.38", "configure_requires", ScopeBuild),
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
			source := sourceForPath(t, result, "Build.PL")
			if source.Detector != "perl-build-pl" || source.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) {
				t.Fatalf("source = %+v", source)
			}
			if !equalDependencies(source.Dependencies, tc.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, tc.want)
			}
		})
	}
}

func TestPerlBuildPLWithoutPrerequisitesIsAbsent(t *testing.T) {
	parser, err := newPerlBuildPLParser(perlBuildPLMatcherConfig{})
	if err != nil {
		t.Fatalf("newPerlBuildPLParser: %v", err)
	}
	result, err := parser.Analyze("Build.PL", []byte("use Module::Build;\nModule::Build->new(name => 'empty')->create_build_script;\n"))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if !result.Recognized || result.Analysis != (SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}) || len(result.Dependencies) != 0 {
		t.Fatalf("result = %+v", result)
	}
}

func perlBuildPLTestDependency(name, constraint, group string, scope DependencyScope) DependencyReference {
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
