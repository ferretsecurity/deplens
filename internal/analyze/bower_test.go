package analyze

import (
	"reflect"
	"testing"
)

func TestBowerParserExtractsSyntheticFixturePatterns(t *testing.T) {
	parser, err := newBowerParser(bowerMatcherConfig{})
	if err != nil {
		t.Fatalf("newBowerParser failed: %v", err)
	}

	tests := []struct {
		name  string
		parts []string
		want  []DependencyReference
	}{
		{
			name:  "development Git shorthand and registry range",
			parts: []string{"js", "bower-dev-git-and-range", "bower.json"},
			want: []DependencyReference{
				{Raw: "test-helper@~2.1.x", Name: "test-helper", VersionConstraint: "~2.1.x", SourceGroup: "devDependencies", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeDevelopment},
				{Raw: "ui-kit@acme/ui-kit#v2.0.0", Name: "ui-kit", SourceGroup: "devDependencies", OriginKind: OriginGit, Relationship: RelationshipDirect, Scope: ScopeDevelopment, Attributes: map[string]string{"source_url": "acme/ui-kit", "source_ref": "v2.0.0"}},
			},
		},
		{
			name:  "runtime registry range",
			parts: []string{"js", "bower-runtime-range", "bower.json"},
			want: []DependencyReference{
				{Raw: "dom-lib@>= 2.0.0", Name: "dom-lib", VersionConstraint: ">= 2.0.0", SourceGroup: "dependencies", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
			},
		},
		{
			name:  "runtime registry pins",
			parts: []string{"js", "bower-runtime-pins", "bower.json"},
			want: []DependencyReference{
				{Raw: "date-tools@4.5.6", Name: "date-tools", VersionConstraint: "4.5.6", SourceGroup: "dependencies", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{Raw: "router@1.2.3", Name: "router", VersionConstraint: "1.2.3", SourceGroup: "dependencies", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := parser.Analyze("bower.json", mustReadTestdataFile(t, tc.parts...))
			if err != nil {
				t.Fatalf("Analyze failed: %v", err)
			}
			if result.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) {
				t.Fatalf("analysis = %+v", result.Analysis)
			}
			if !reflect.DeepEqual(result.Dependencies, tc.want) {
				t.Fatalf("dependencies = %#v, want %#v", result.Dependencies, tc.want)
			}
		})
	}
}

func TestBowerParserReportsConclusiveEmpty(t *testing.T) {
	parser, err := newBowerParser(bowerMatcherConfig{})
	if err != nil {
		t.Fatalf("newBowerParser failed: %v", err)
	}

	result, err := parser.Analyze("bower.json", mustReadTestdataFile(t, "js", "bower-no-deps", "bower.json"))
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}
	if result.Analysis != (SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}) {
		t.Fatalf("analysis = %+v", result.Analysis)
	}
	if len(result.Dependencies) != 0 {
		t.Fatalf("dependencies = %+v, want none", result.Dependencies)
	}
}
