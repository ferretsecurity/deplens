package analyze

import (
	"maps"
	"path/filepath"
	"testing"
)

func TestScanRequirementsSeparatesPipSyntaxFromVersions(t *testing.T) {
	result, err := Scan(filepath.Join("..", "..", "testdata", "python", "requirements-pip-syntax"), nil, mustLoadDefaultRules(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("sources = %+v", result.Sources)
	}
	source := result.Sources[0]
	if source.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) || len(source.Diagnostics) != 0 {
		t.Fatalf("analysis = %+v, diagnostics = %+v", source.Analysis, source.Diagnostics)
	}
	want := []DependencyReference{
		{Raw: "requests==2.31.0 --hash=sha256:aaa --hash sha256:bbb", Name: "requests", OriginKind: OriginRegistry, VersionConstraint: "==2.31.0", VERS: "vers:pypi/2.31.0", Attributes: map[string]string{"hashes": "sha256:aaa,sha256:bbb"}},
		{Raw: `fastapi[all]>=0.110; python_version >= "3.10" --hash=sha256:ccc`, Name: "fastapi", OriginKind: OriginRegistry, VersionConstraint: ">=0.110", VERS: "vers:pypi/>=0.110", Attributes: map[string]string{"extras": "all", "marker": `python_version >= "3.10"`, "hashes": "sha256:ccc"}},
		{Raw: "-e git+https://example.test/requests.git@v2.31.0#egg=requests", Name: "requests", OriginKind: OriginGit, Attributes: map[string]string{"editable": "true", "source_url": "git+https://example.test/requests.git", "source_ref": "v2.31.0"}},
		{Raw: "httpx @ https://example.test/httpx-0.27.0.zip#sha256=ddd", Name: "httpx", OriginKind: OriginURL, Attributes: map[string]string{"source_url": "https://example.test/httpx-0.27.0.zip#sha256=ddd"}},
		{Raw: `demo[cli] @ git+ssh://git@example.test/demo.git@release#subdirectory=python ; os_name == "posix"`, Name: "demo", OriginKind: OriginGit, Attributes: map[string]string{"extras": "cli", "source_url": "git+ssh://git@example.test/demo.git#subdirectory=python", "source_ref": "release", "marker": `os_name == "posix"`}},
		{Raw: "-e ../local-project", OriginKind: OriginPath, Attributes: map[string]string{"editable": "true", "source_path": "../local-project"}},
		{Raw: "./dist/demo-1.0.whl", OriginKind: OriginPath, Attributes: map[string]string{"source_path": "./dist/demo-1.0.whl"}},
		{Raw: "https://example.test/dist/archive.whl#sha256=eee", OriginKind: OriginURL, Attributes: map[string]string{"source_url": "https://example.test/dist/archive.whl#sha256=eee"}},
		{Raw: "urllib3<3", Name: "urllib3", OriginKind: OriginRegistry, VersionConstraint: "<3", VERS: "vers:pypi/<3"},
		{Raw: "urllib3<3", Name: "urllib3", OriginKind: OriginRegistry, VersionConstraint: "<3", VERS: "vers:pypi/<3"},
	}
	if len(source.Dependencies) != len(want) {
		t.Fatalf("dependencies = %+v, want %d references", source.Dependencies, len(want))
	}
	for i, expected := range want {
		got := source.Dependencies[i]
		if got.PackageType != "pypi" || got.Raw != expected.Raw || got.Name != expected.Name || got.VersionConstraint != expected.VersionConstraint || got.VERS != expected.VERS || got.OriginKind != expected.OriginKind || !maps.Equal(got.Attributes, expected.Attributes) {
			t.Errorf("dependency %d = %+v, want %+v", i, got, expected)
		}
	}
}
