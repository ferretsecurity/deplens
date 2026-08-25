package analyze

import (
	"path/filepath"
	"testing"
)

func TestClojureDepsEDNFixturesExtractDependencies(t *testing.T) {
	tests := []struct {
		name     string
		fixture  string
		analysis SourceAnalysis
		want     []DependencyReference
	}{
		{
			name:     "paths without dependencies",
			fixture:  "deps-edn-paths-only",
			analysis: SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete},
			want:     []DependencyReference{},
		},
		{
			name:     "main and alias dependencies",
			fixture:  "deps-edn-main-and-aliases",
			analysis: SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete},
			want: []DependencyReference{
				{PackageType: "maven", Raw: "tools/build@v0.10.7", Name: "tools/build", SourceGroup: "aliases.build.deps", OriginKind: OriginGit, Relationship: RelationshipDirect, Scope: ScopeBuild, Attributes: map[string]string{"source_ref": "573711e", "source_ref_kind": "revision", "source_tag": "v0.10.7"}},
				{PackageType: "maven", Raw: "org.clojure/test.check@1.1.3", Name: "org.clojure/test.check", VersionConstraint: "[1.1.3]", VERS: "vers:maven/1.1.3", SourceGroup: "aliases.test.extra-deps", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeTest},
				{PackageType: "maven", Raw: "org.clojure/clojure@1.12.4", Name: "org.clojure/clojure", VersionConstraint: "[1.12.4]", VERS: "vers:maven/1.12.4", SourceGroup: "deps", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{PackageType: "maven", Raw: "ring/ring-core@1.15.3", Name: "ring/ring-core", VersionConstraint: "[1.15.3]", VERS: "vers:maven/1.15.3", SourceGroup: "deps", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
			},
		},
		{
			name:     "alias dependency coordinate kinds",
			fixture:  "deps-edn-alias-coordinate-kinds",
			analysis: SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete},
			want: []DependencyReference{
				{PackageType: "maven", Raw: "tools/build@v0.9.6", Name: "tools/build", SourceGroup: "aliases.build.deps", OriginKind: OriginGit, Relationship: RelationshipDirect, Scope: ScopeBuild, Attributes: map[string]string{"source_ref": "8e78bcc", "source_ref_kind": "revision", "source_tag": "v0.9.6"}},
				{PackageType: "maven", Raw: "deploy/tool@0.1.5", Name: "deploy/tool", VersionConstraint: "[0.1.5]", VERS: "vers:maven/0.1.5", SourceGroup: "aliases.deploy.replace-deps", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeBuild},
				{PackageType: "maven", Raw: "org.clojure/clojure@1.11.4", Name: "org.clojure/clojure", VersionConstraint: "[1.11.4]", VERS: "vers:maven/1.11.4", SourceGroup: "aliases.dev.extra-deps", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeDevelopment},
				{PackageType: "maven", Raw: "logback/classic@1.1.3", Name: "logback/classic", VersionConstraint: "[1.1.3]", VERS: "vers:maven/1.1.3", SourceGroup: "aliases.test.extra-deps", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeTest},
				{PackageType: "maven", Raw: "test/runner@https://example.test/runner.git", Name: "test/runner", SourceGroup: "aliases.test.extra-deps", OriginKind: OriginGit, Relationship: RelationshipDirect, Scope: ScopeTest, Attributes: map[string]string{"source_url": "https://example.test/runner.git", "source_ref": "209b645", "source_ref_kind": "revision"}},
			},
		},
	}

	ruleset := mustLoadDefaultRules(t)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := Scan(filepath.Join("..", "..", "testdata", "clojure", test.fixture), nil, ruleset)
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			source := sourceForPath(t, result, "deps.edn")
			if source.Detector != "clojure-deps-edn" || source.Analysis != test.analysis {
				t.Fatalf("source = %+v", source)
			}
			if !equalDependencies(source.Dependencies, test.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, test.want)
			}
		})
	}
}

func TestClojureDepsEDNRejectsMalformedEDN(t *testing.T) {
	parser, err := newClojureDepsEDNParser(clojureDepsEDNMatcherConfig{})
	if err != nil {
		t.Fatalf("newClojureDepsEDNParser: %v", err)
	}
	if _, err := parser.Analyze("deps.edn", []byte(`{:deps {example/lib {:mvn/version "1.2.3"}}`)); err == nil {
		t.Fatal("expected malformed deps.edn error")
	}
}

func TestClojureDepsEDNExtractsLegacyGitSHA(t *testing.T) {
	parser, err := newClojureDepsEDNParser(clojureDepsEDNMatcherConfig{})
	if err != nil {
		t.Fatalf("newClojureDepsEDNParser: %v", err)
	}

	result, err := parser.Analyze("deps.edn", []byte(`{:aliases {:test {:extra-deps {example/tool {:git/url "https://example.test/tool.git" :sha "abc123"}}}}}`))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	want := []DependencyReference{
		{Raw: "example/tool@https://example.test/tool.git", Name: "example/tool", SourceGroup: "aliases.test.extra-deps", OriginKind: OriginGit, Relationship: RelationshipDirect, Scope: ScopeTest, Attributes: map[string]string{"source_url": "https://example.test/tool.git", "source_ref": "abc123", "source_ref_kind": "revision"}},
	}
	if result.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) || !equalDependencies(result.Dependencies, want) {
		t.Fatalf("result = %+v, want dependencies %#v", result, want)
	}
}

func TestClojureDepsEDNAcceptsLiteralNewlinesInStrings(t *testing.T) {
	forms, err := parseClojureForms([]byte("\"first\nsecond\""))
	if err != nil {
		t.Fatalf("parseClojureForms: %v", err)
	}
	if len(forms) != 1 || clojureNodeValue(forms[0]) != "first\nsecond" {
		t.Fatalf("forms = %#v", forms)
	}
}
