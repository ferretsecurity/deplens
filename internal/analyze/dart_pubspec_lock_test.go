package analyze

import (
	"path/filepath"
	"testing"
)

func TestDartPubspecLockFixturesExtractDependencyReferences(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		want    []DependencyReference
	}{
		{
			name:    "legacy hosted and Git entries",
			fixture: "pubspec-lock-legacy-hosted-and-git",
			want: []DependencyReference{
				{PackageType: "pub", Raw: "legacy_git@0.6.2", Name: "legacy_git", Version: "0.6.2", SourceGroup: "packages", OriginKind: OriginGit, Relationship: RelationshipInconclusive, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://example.test/legacy.git", "source_ref": "abc123", "source_ref_kind": "revision"}},
				{PackageType: "pub", Raw: "legacy_hosted@0.8.10+3", Name: "legacy_hosted", Version: "0.8.10+3", SourceGroup: "packages", OriginKind: OriginRegistry, Relationship: RelationshipInconclusive, Scope: ScopeRuntime},
			},
		},
		{
			name:    "modern hosted direct development and transitive entries",
			fixture: "pubspec-lock-modern-hosted",
			want: []DependencyReference{
				{PackageType: "pub", Raw: "direct_dev@2.0.0", Name: "direct_dev", Version: "2.0.0", SourceGroup: "packages", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeDevelopment, Attributes: map[string]string{"hosted_url": "https://pub.dev", "sha256": "deadbeef"}},
				{PackageType: "pub", Raw: "transitive@1.0.0", Name: "transitive", Version: "1.0.0", SourceGroup: "packages", OriginKind: OriginRegistry, Relationship: RelationshipTransitive, Scope: ScopeRuntime, Attributes: map[string]string{"hosted_url": "https://pub.dev"}},
			},
		},
		{
			name:    "modern hosted and SDK entries",
			fixture: "pubspec-lock-modern-hosted-and-sdk",
			want: []DependencyReference{
				{PackageType: "pub", Raw: "flutter@0.0.0", Name: "flutter", Version: "0.0.0", SourceGroup: "packages", Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"sdk": "flutter"}},
				{PackageType: "pub", Raw: "http@1.1.2", Name: "http", Version: "1.1.2", SourceGroup: "packages", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"hosted_url": "https://pub.dev"}},
			},
		},
	}

	ruleset := mustLoadDefaultRules(t)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Scan(filepath.Join("..", "..", "testdata", "dart", tc.fixture), nil, ruleset)
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			source := sourceForPath(t, result, "pubspec.lock")
			if source.Detector != "dart-pubspec-lock" || source.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) {
				t.Fatalf("source = %+v", source)
			}
			if !equalDependencies(source.Dependencies, tc.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, tc.want)
			}
		})
	}
}

func TestDartPubspecLockWithoutPackagesIsNotRecognized(t *testing.T) {
	parser, err := newDartPubspecLockParser(dartPubspecLockMatcherConfig{})
	if err != nil {
		t.Fatalf("newDartPubspecLockParser: %v", err)
	}
	result, err := parser.Analyze("pubspec.lock", []byte("sdks:\n  dart: '>=3.0.0 <4.0.0'\n"))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.Recognized {
		t.Fatalf("result = %+v", result)
	}
}

func TestDartPubspecLockWithNoDependenciesIsCompleteAndAbsent(t *testing.T) {
	parser, err := newDartPubspecLockParser(dartPubspecLockMatcherConfig{})
	if err != nil {
		t.Fatalf("newDartPubspecLockParser: %v", err)
	}
	result, err := parser.Analyze("pubspec.lock", []byte("packages: {}\n"))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if !result.Recognized || result.Analysis != (SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}) || len(result.Dependencies) != 0 {
		t.Fatalf("result = %+v", result)
	}
}
