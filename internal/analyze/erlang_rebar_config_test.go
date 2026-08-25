package analyze

import (
	"path/filepath"
	"testing"
)

func TestErlangRebarConfigFixturesExtractDependencies(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		want    []DependencyReference
	}{
		{
			name:    "registry and Git dependencies",
			fixture: "rebar-config-registry-git",
			want: []DependencyReference{
				{PackageType: "hex", Raw: "ehttpc@https://github.com/emqx/ehttpc", Name: "ehttpc", SourceGroup: "deps", OriginKind: OriginGit, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://github.com/emqx/ehttpc", "source_ref": "0.7.1", "source_ref_kind": "tag"}},
				{PackageType: "hex", Raw: "erliam@1.0.1", Name: "erliam", VersionConstraint: "1.0.1", SourceGroup: "deps", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{PackageType: "hex", Raw: "rebar3_hex@~> 7.0.7", Name: "rebar3_hex", VersionConstraint: "~> 7.0.7", SourceGroup: "project_plugins", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeBuild},
			},
		},
		{
			name:    "bare plugin",
			fixture: "rebar-config-bare-plugin",
			want: []DependencyReference{
				{PackageType: "hex", Raw: "esq@2.0.7", Name: "esq", VersionConstraint: "2.0.7", SourceGroup: "deps", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{PackageType: "hex", Raw: "jsone@1.9.0", Name: "jsone", VersionConstraint: "1.9.0", SourceGroup: "deps", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{PackageType: "hex", Raw: "rebar3_hex", Name: "rebar3_hex", SourceGroup: "project_plugins", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeBuild},
			},
		},
		{
			name:    "aliases Git subdirectories and profiles",
			fixture: "rebar-config-profiles",
			want: []DependencyReference{
				{PackageType: "hex", Raw: "gproc", Name: "gproc", SourceGroup: "deps", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{PackageType: "hex", Raw: "itc@1.0.0", Name: "interval_tree_clocks", VersionConstraint: "1.0.0", SourceGroup: "deps", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"declared_name": "itc"}},
				{PackageType: "hex", Raw: "opentelemetry_api@https://github.com/open-telemetry/opentelemetry-erlang", Name: "opentelemetry_api", SourceGroup: "deps", OriginKind: OriginGit, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://github.com/open-telemetry/opentelemetry-erlang", "source_ref": "main", "source_ref_kind": "branch", "subdir": "apps/opentelemetry_api"}},
				{PackageType: "hex", Raw: "tak@https://github.com/ferd/tak.git", Name: "tak", SourceGroup: "deps", OriginKind: OriginGit, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://github.com/ferd/tak.git", "source_ref": "master", "source_ref_kind": "branch"}},
				{PackageType: "hex", Raw: "meck", Name: "meck", SourceGroup: "profiles.test.deps", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeTest},
				{PackageType: "hex", Raw: "proper", Name: "proper", SourceGroup: "profiles.test.deps", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeTest},
			},
		},
	}

	ruleset := mustLoadDefaultRules(t)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := Scan(filepath.Join("..", "..", "testdata", "erlang", test.fixture), nil, ruleset)
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			source := sourceForPath(t, result, "rebar.config")
			if source.Detector != "erlang-rebar-config" || source.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) {
				t.Fatalf("source = %+v", source)
			}
			if !equalDependencies(source.Dependencies, test.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, test.want)
			}
		})
	}
}

func TestErlangRebarConfigRejectsMalformedTerms(t *testing.T) {
	parser, err := newErlangRebarConfigParser(erlangRebarConfigMatcherConfig{})
	if err != nil {
		t.Fatalf("newErlangRebarConfigParser: %v", err)
	}
	if _, err := parser.Analyze("rebar.config", []byte(`{deps, [broken]`)); err == nil {
		t.Fatal("expected malformed rebar.config error")
	}
}
