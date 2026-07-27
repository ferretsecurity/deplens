package analyze

import (
	"reflect"
	"strings"
	"testing"
)

func TestPackageJSONExtractsGroupsAliasesAndOrigins(t *testing.T) {
	parser, err := newPackageJSONParser(packageJSONMatcherConfig{})
	if err != nil {
		t.Fatalf("newPackageJSONParser failed: %v", err)
	}
	result, err := parser.Analyze("package.json", []byte(`{
  "packageManager": "pnpm@10.12.0",
  "workspaces": ["apps/*", "packages/*"],
  "dependencies": {
    "@acme/ui": "workspace:^",
    "local-plugin": "file:../local-plugin",
    "react": "^19.0.0",
    "server": "npm:@acme/server@^3.2.0",
    "theme": "https://packages.example.com/theme.tgz",
    "toolkit": "github:acme/toolkit#v2.4.0"
  },
  "devDependencies": {"react": "^18.0.0", "typescript": "~5.8.0"},
  "peerDependencies": {"loose-peer": "^1.0.0", "react-dom": "^19.0.0"},
  "peerDependenciesMeta": {"loose-peer": "invalid", "react-dom": {"optional": true}},
  "optionalDependencies": {"fsevents": "^2.3.3"}
}`))
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}
	if result.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) {
		t.Fatalf("unexpected analysis: %+v", result.Analysis)
	}

	want := []DependencyReference{
		{Raw: "@acme/ui@workspace:^", Name: "@acme/ui", VersionConstraint: "^", SourceGroup: "dependencies", OriginKind: OriginWorkspace, Relationship: RelationshipDirect, Scope: ScopeRuntime},
		{Raw: "local-plugin@file:../local-plugin", Name: "local-plugin", SourceGroup: "dependencies", OriginKind: OriginPath, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"path": "../local-plugin", "protocol": "file"}},
		{Raw: "react@^19.0.0", Name: "react", VersionConstraint: "^19.0.0", SourceGroup: "dependencies", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
		{Raw: "server@npm:@acme/server@^3.2.0", Name: "@acme/server", VersionConstraint: "^3.2.0", SourceGroup: "dependencies", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"declared_name": "server"}},
		{Raw: "theme@https://packages.example.com/theme.tgz", Name: "theme", SourceGroup: "dependencies", OriginKind: OriginURL, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://packages.example.com/theme.tgz"}},
		{Raw: "toolkit@github:acme/toolkit#v2.4.0", Name: "toolkit", SourceGroup: "dependencies", OriginKind: OriginGit, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_ref": "v2.4.0", "source_url": "github:acme/toolkit"}},
		{Raw: "react@^18.0.0", Name: "react", VersionConstraint: "^18.0.0", SourceGroup: "devDependencies", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeDevelopment},
		{Raw: "typescript@~5.8.0", Name: "typescript", VersionConstraint: "~5.8.0", SourceGroup: "devDependencies", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeDevelopment},
		{Raw: "loose-peer@^1.0.0", Name: "loose-peer", VersionConstraint: "^1.0.0", SourceGroup: "peerDependencies", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
		{Raw: "react-dom@^19.0.0", Name: "react-dom", VersionConstraint: "^19.0.0", SourceGroup: "peerDependencies", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeOptional},
		{Raw: "fsevents@^2.3.3", Name: "fsevents", VersionConstraint: "^2.3.3", SourceGroup: "optionalDependencies", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeOptional},
	}
	if !reflect.DeepEqual(result.Dependencies, want) {
		t.Fatalf("dependencies mismatch:\n got: %#v\nwant: %#v", result.Dependencies, want)
	}
}

func TestPackageJSONReportsPartialExtraction(t *testing.T) {
	parser, _ := newPackageJSONParser(packageJSONMatcherConfig{})
	result, err := parser.Analyze("package.json", []byte(`{
  "dependencies": {
    "react": "^19.0.0",
    "broken": {"version": "1.0.0"}
  },
  "devDependencies": "typescript"
}`))
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}
	if result.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionPartial}) {
		t.Fatalf("unexpected analysis: %+v", result.Analysis)
	}
	if len(result.Dependencies) != 1 || result.Dependencies[0].Raw != "react@^19.0.0" {
		t.Fatalf("unexpected dependencies: %+v", result.Dependencies)
	}
	gotMessages := []string{result.Diagnostics[0].Message, result.Diagnostics[1].Message}
	wantMessages := []string{
		"dependencies.broken: expected a string dependency specifier",
		"devDependencies: expected an object of dependency specifiers",
	}
	if !reflect.DeepEqual(gotMessages, wantMessages) {
		t.Fatalf("diagnostics mismatch: got %q want %q", gotMessages, wantMessages)
	}
}

func TestPackageJSONHandlesEmptyMalformedAndUnknownSpecifiers(t *testing.T) {
	parser, _ := newPackageJSONParser(packageJSONMatcherConfig{})
	tests := []struct {
		name       string
		content    string
		analysis   SourceAnalysis
		dependency *DependencyReference
	}{
		{
			name:     "empty",
			content:  `{"dependencies":{},"devDependencies":null,"workspaces":["packages/*"]}`,
			analysis: SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete},
		},
		{
			name:     "malformed only",
			content:  `{"dependencies":{"broken":42}}`,
			analysis: SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionUnsupported},
		},
		{
			name:     "empty dependency name",
			content:  `{"dependencies":{"":"^1.0.0"}}`,
			analysis: SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionUnsupported},
		},
		{
			name:     "exact registry version",
			content:  `{"dependencies":{"exact":"1.2.3"}}`,
			analysis: SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete},
			dependency: &DependencyReference{
				Raw: "exact@1.2.3", Name: "exact", VersionConstraint: "1.2.3", SourceGroup: "dependencies",
				OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime,
			},
		},
		{
			name:     "registry tag",
			content:  `{"dependencies":{"tagged":"latest"}}`,
			analysis: SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete},
			dependency: &DependencyReference{
				Raw: "tagged@latest", Name: "tagged", VersionConstraint: "latest", SourceGroup: "dependencies",
				OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime,
			},
		},
		{
			name:     "unknown protocol",
			content:  `{"dependencies":{"catalogued":"catalog:frontend"}}`,
			analysis: SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete},
			dependency: &DependencyReference{
				Raw: "catalogued@catalog:frontend", Name: "catalogued",
				SourceGroup: "dependencies", Relationship: RelationshipDirect, Scope: ScopeRuntime,
			},
		},
		{
			name:     "relative path",
			content:  `{"dependencies":{"local":"../local-package"}}`,
			analysis: SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete},
			dependency: &DependencyReference{
				Raw: "local@../local-package", Name: "local", SourceGroup: "dependencies",
				OriginKind: OriginPath, Relationship: RelationshipDirect, Scope: ScopeRuntime,
				Attributes: map[string]string{"path": "../local-package"},
			},
		},
		{
			name:     "hosted Git shorthand",
			content:  `{"dependencies":{"hosted":"owner/repository#v1"}}`,
			analysis: SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete},
			dependency: &DependencyReference{
				Raw: "hosted@owner/repository#v1", Name: "hosted", SourceGroup: "dependencies",
				OriginKind: OriginGit, Relationship: RelationshipDirect, Scope: ScopeRuntime,
				Attributes: map[string]string{"source_ref": "v1", "source_url": "owner/repository"},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := parser.Analyze("package.json", []byte(test.content))
			if err != nil {
				t.Fatalf("Analyze failed: %v", err)
			}
			if result.Analysis != test.analysis {
				t.Fatalf("analysis = %+v, want %+v", result.Analysis, test.analysis)
			}
			if test.dependency != nil && (len(result.Dependencies) != 1 || !reflect.DeepEqual(result.Dependencies[0], *test.dependency)) {
				t.Fatalf("dependency = %#v, want %#v", result.Dependencies, *test.dependency)
			}
		})
	}
}

func TestPackageJSONRejectsInvalidJSONAndConfiguration(t *testing.T) {
	parser, _ := newPackageJSONParser(packageJSONMatcherConfig{})
	if _, err := parser.Analyze("package.json", []byte(`{"dependencies":`)); err == nil || !strings.Contains(err.Error(), "parse package.json") {
		t.Fatalf("expected package.json parse error, got %v", err)
	}

	_, err := loadRules("test.yaml", []byte(`rules:
  - id: custom-package
    package-type: npm
    form: manifest
    roles: [declaration, constraint]
    filename-regex: '^package\.custom\.json$'
    analyzer:
      type: package-json
      groups: [dependencies]
`))
	if err == nil || !strings.Contains(err.Error(), "field groups not found") {
		t.Fatalf("expected strict package-json configuration error, got %v", err)
	}
}
