package analyze

import (
	"path/filepath"
	"testing"
)

func TestCondaEnvironmentFixturesExtractDependencyReferences(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		path    string
		want    []DependencyReference
	}{
		{
			name:    "Conda and pip declarations",
			fixture: "conda-env-alt-pip",
			path:    "conda.yaml",
			want: []DependencyReference{
				condaEnvironmentTestDependency("pip", "pip=22.1.2", "=22.1.2"),
				condaEnvironmentTestDependency("python", "python=3.9.13", "=3.9.13"),
				{PackageType: "pypi", Raw: "rpaframework==15.1.1", Name: "rpaframework", VersionConstraint: "==15.1.1", SourceGroup: "dependencies.pip", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
			},
		},
		{
			name:    "exported environment with multiple pip packages",
			fixture: "conda-env-alt-export",
			path:    "conda.yaml",
			want: []DependencyReference{
				condaEnvironmentTestDependency("Pillow", "Pillow=9.2.0", "=9.2.0"),
				condaEnvironmentTestDependency("pytorch", "pytorch=1.12.1", "=1.12.1"),
				{PackageType: "pypi", Raw: "Augmentor==0.2.10", Name: "Augmentor", VersionConstraint: "==0.2.10", SourceGroup: "dependencies.pip", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{PackageType: "pypi", Raw: "pyyaml==6.0", Name: "pyyaml", VersionConstraint: "==6.0", SourceGroup: "dependencies.pip", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
			},
		},
		{
			name:    "development environment with plain and range specs",
			fixture: "conda-env-alt-development",
			path:    filepath.ToSlash(filepath.Join("dev", "conda.yaml")),
			want: []DependencyReference{
				condaEnvironmentTestDependency("cmake", "cmake>=3.15", ">=3.15"),
				condaEnvironmentTestDependency("make", "make", ""),
				condaEnvironmentTestDependency("pyscipopt", "pyscipopt >= 3.0.1", ">= 3.0.1"),
			},
		},
	}

	ruleset := mustLoadDefaultRules(t)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Scan(filepath.Join("..", "..", "testdata", "python", tc.fixture), nil, ruleset)
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			source := sourceForPath(t, result, tc.path)
			if source.Detector != "python-conda-env-alt" || source.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) {
				t.Fatalf("source = %+v", source)
			}
			applyDependencyVERS(tc.want)
			if !equalDependencies(source.Dependencies, tc.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, tc.want)
			}
		})
	}
}

func TestCondaEnvironmentWithoutDependenciesIsCompleteAndAbsent(t *testing.T) {
	parser, err := newCondaEnvironmentParser(struct{}{})
	if err != nil {
		t.Fatalf("newCondaEnvironmentParser: %v", err)
	}
	result, err := parser.Analyze("conda.yaml", []byte("name: empty\nchannels:\n  - conda-forge\n"))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.Analysis != (SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}) || len(result.Dependencies) != 0 {
		t.Fatalf("result = %+v", result)
	}
}

func condaEnvironmentTestDependency(name, raw, constraint string) DependencyReference {
	return DependencyReference{
		PackageType:       "conda",
		Raw:               raw,
		Name:              name,
		VersionConstraint: constraint,
		SourceGroup:       "dependencies",
		OriginKind:        OriginRegistry,
		Relationship:      RelationshipDirect,
		Scope:             ScopeRuntime,
	}
}
