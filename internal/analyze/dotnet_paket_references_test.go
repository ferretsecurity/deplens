package analyze

import (
	"path/filepath"
	"testing"
)

func TestDotnetPaketReferencesSemanticFixtures(t *testing.T) {
	tests := []struct {
		name       string
		fixtureDir string
		want       []DependencyReference
	}{
		{
			name:       "default group",
			fixtureDir: "paket-references-default",
			want: []DependencyReference{
				paketReferenceDependency("FSharp.Core", "default"),
				paketReferenceDependency("Newtonsoft.Json", "default"),
				paketReferenceDependency("System.Net.Http", "default"),
			},
		},
		{
			name:       "named group",
			fixtureDir: "paket-references-fake-group",
			want: []DependencyReference{
				paketReferenceDependency("Fake.Core.Target", "fake"),
				paketReferenceDependency("Fake.DotNet.Cli", "fake"),
				paketReferenceDependency("Fake.IO.FileSystem", "fake"),
			},
		},
		{
			name:       "file directive",
			fixtureDir: "paket-references-file-directive",
			want: []DependencyReference{
				paketReferenceDependency("FSharp.Core", "default"),
				paketReferenceDependency("Microsoft.CodeAnalysis.CSharp", "default"),
			},
		},
		{
			name:       "package settings and optional prefix",
			fixtureDir: "paket-references-package-settings",
			want: []DependencyReference{
				paketReferenceDependency("Contoso.Test", "Tests"),
				paketReferenceDependency("Contoso.Runtime", "default"),
				paketReferenceDependency("Contoso.Build", "fcs"),
			},
		},
	}

	ruleset := mustLoadDefaultRules(t)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Scan(filepath.Join("..", "..", "testdata", "dotnet", tc.fixtureDir), nil, ruleset)
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			source := sourceForPath(t, result, "paket.references")
			if source.Detector != "dotnet-paket-references" || source.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) {
				t.Fatalf("source = %+v", source)
			}
			if !equalDependencies(source.Dependencies, tc.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, tc.want)
			}
		})
	}
}

func TestDotnetPaketReferencesWithoutReferencesIsComplete(t *testing.T) {
	parser, err := newDotnetPaketReferencesParser(dotnetPaketReferencesMatcherConfig{})
	if err != nil {
		t.Fatalf("newDotnetPaketReferencesParser: %v", err)
	}
	result, err := parser.Analyze("paket.references", []byte("File: script.fsx\n"))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if !result.Recognized || result.Analysis != (SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}) || len(result.Dependencies) != 0 {
		t.Fatalf("result = %+v", result)
	}
}

func paketReferenceDependency(name, group string) DependencyReference {
	return DependencyReference{
		PackageType:  "nuget",
		Raw:          name,
		Name:         name,
		SourceGroup:  group,
		OriginKind:   OriginRegistry,
		Relationship: RelationshipDirect,
		Scope:        paketGroupScope(group),
	}
}
