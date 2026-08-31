package analyze

import (
	"path/filepath"
	"testing"
)

func TestErlangRebarLockFixturesExtractDependencies(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		want    []DependencyReference
	}{
		{
			name:    "version 1.1 Hex dependency",
			fixture: "rebar-lock-v11-hex",
			want: []DependencyReference{
				{PackageType: "hex", Raw: "certifi@2.5.1", Name: "certifi", Version: "2.5.1", SourceGroup: "dependencies", OriginKind: OriginRegistry, Relationship: RelationshipTransitive, Scope: ScopeRuntime},
			},
		},
		{
			name:    "version 1.2 Hex and Git dependencies",
			fixture: "rebar-lock-v12-mixed",
			want: []DependencyReference{
				{PackageType: "hex", Raw: "cowlib@2.13.0", Name: "cowlib", Version: "2.13.0", SourceGroup: "dependencies", OriginKind: OriginRegistry, Relationship: RelationshipTransitive, Scope: ScopeRuntime},
				{PackageType: "hex", Raw: "ehttpc@c18e5efacde5556bb8bf86b1bc1e35a85a67928d", Name: "ehttpc", Version: "c18e5efacde5556bb8bf86b1bc1e35a85a67928d", SourceGroup: "dependencies", OriginKind: OriginGit, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://github.com/emqx/ehttpc", "source_ref": "c18e5efacde5556bb8bf86b1bc1e35a85a67928d", "source_ref_kind": "ref"}},
			},
		},
		{
			name:    "legacy Git dependency",
			fixture: "rebar-lock-legacy-git",
			want: []DependencyReference{
				{PackageType: "hex", Raw: "goldrush@8f1b715d36b650ec1e1f5612c00e28af6ab0de82", Name: "goldrush", Version: "8f1b715d36b650ec1e1f5612c00e28af6ab0de82", SourceGroup: "dependencies", OriginKind: OriginGit, Relationship: RelationshipTransitive, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://github.com/basho/goldrush.git", "source_ref": "8f1b715d36b650ec1e1f5612c00e28af6ab0de82", "source_ref_kind": "ref"}},
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
			source := sourceForPath(t, result, "rebar.lock")
			if source.Detector != "erlang-rebar-lock" || source.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) {
				t.Fatalf("source = %+v", source)
			}
			if !equalDependencies(source.Dependencies, test.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, test.want)
			}
		})
	}
}

func TestErlangRebarLockEmptyListHasCompleteEmptyResult(t *testing.T) {
	parser, err := newErlangRebarLockParser(erlangRebarLockMatcherConfig{})
	if err != nil {
		t.Fatalf("newErlangRebarLockParser: %v", err)
	}
	result, err := parser.Analyze("rebar.lock", []byte("[].\n"))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if !result.Recognized || result.Analysis != (SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}) || len(result.Dependencies) != 0 {
		t.Fatalf("result = %+v", result)
	}
}
