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
		{
			name:    "environment file with Conda and pip package declarations",
			fixture: "conda-environment-pytorch",
			path:    "environment.yaml",
			want: []DependencyReference{
				condaEnvironmentTestDependency("cudatoolkit", "cudatoolkit=10.2", "=10.2"),
				condaEnvironmentTestDependency("python", "python", ""),
				condaEnvironmentTestDependency("pytorch", "pytorch", ""),
				{PackageType: "pypi", Raw: "ipdb", Name: "ipdb", SourceGroup: "dependencies.pip", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{PackageType: "pypi", Raw: "scikit-image", Name: "scikit-image", SourceGroup: "dependencies.pip", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
			},
		},
		{
			name:    "environment file with a pip requirements-file reference",
			fixture: "conda-environment-pip-requirements",
			path:    "environment.yaml",
			want: []DependencyReference{
				condaEnvironmentTestDependency("anaconda", "anaconda", ""),
				condaEnvironmentTestDependency("pip", "pip", ""),
				condaEnvironmentTestDependency("python", "python==3.9", "==3.9"),
				{PackageType: "generic", Raw: "-r file:requirements.txt", Name: "requirements.txt", SourceGroup: "dependencies.pip", OriginKind: OriginPath, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"path": "requirements.txt"}},
			},
		},
		{
			name:    "environment file with spaced Conda pin",
			fixture: "conda-environment-metaphlan",
			path:    "environment.yaml",
			want: []DependencyReference{
				condaEnvironmentTestDependency("metaphlan", "metaphlan = 4.2.4", "= 4.2.4"),
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
			if source.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) {
				t.Fatalf("source = %+v", source)
			}
			if tc.fixture == "conda-environment-pytorch" || tc.fixture == "conda-environment-pip-requirements" || tc.fixture == "conda-environment-metaphlan" {
				if source.Detector != "python-conda-environment" {
					t.Fatalf("detector = %q, want python-conda-environment", source.Detector)
				}
			} else if source.Detector != "python-conda-env-alt" {
				t.Fatalf("detector = %q, want python-conda-env-alt", source.Detector)
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
