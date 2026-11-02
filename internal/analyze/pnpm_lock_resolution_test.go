package analyze

import "testing"

func TestPNPMLockNormalizesImporterResolutions(t *testing.T) {
	tests := []struct {
		name      string
		version   string
		specifier string
		want      DependencyReference
	}{
		{
			name: "peer context", version: "1.0.0(typescript@5.2.2)(react@18.2.0)", specifier: "^1.0.0",
			want: DependencyReference{Raw: "declared@1.0.0(typescript@5.2.2)(react@18.2.0)", Name: "declared", Version: "1.0.0", SourceGroup: "dependencies", Attributes: map[string]string{"specifier": "^1.0.0", "peer_context": "(typescript@5.2.2)(react@18.2.0)"}},
		},
		{
			name: "legacy peer context", version: "1.0.0_react@18.2.0",
			want: DependencyReference{Raw: "declared@1.0.0_react@18.2.0", Name: "declared", Version: "1.0.0", SourceGroup: "dependencies", Attributes: map[string]string{"peer_context": "_react@18.2.0"}},
		},
		{
			name: "nested peer context", version: "1.0.0(react-dom@18.2.0(react@18.2.0))",
			want: DependencyReference{Raw: "declared@1.0.0(react-dom@18.2.0(react@18.2.0))", Name: "declared", Version: "1.0.0", SourceGroup: "dependencies", Attributes: map[string]string{"peer_context": "(react-dom@18.2.0(react@18.2.0))"}},
		},
		{
			name: "alias", version: "react@18.2.0", specifier: "npm:react@^18",
			want: DependencyReference{Raw: "declared@react@18.2.0", Name: "react", Version: "18.2.0", SourceGroup: "dependencies", Attributes: map[string]string{"specifier": "npm:react@^18", "declared_name": "declared"}},
		},
		{
			name: "scoped alias and peer context", version: "@scope/real@2.3.4(react@18.2.0)", specifier: "npm:@scope/real@^2",
			want: DependencyReference{Raw: "declared@@scope/real@2.3.4(react@18.2.0)", Name: "@scope/real", Version: "2.3.4", SourceGroup: "dependencies", Attributes: map[string]string{"specifier": "npm:@scope/real@^2", "declared_name": "declared", "peer_context": "(react@18.2.0)"}},
		},
		{
			name: "alias specifier with bare version", version: "2.3.4", specifier: "npm:@scope/real@^2",
			want: DependencyReference{Raw: "declared@2.3.4", Name: "@scope/real", Version: "2.3.4", SourceGroup: "dependencies", Attributes: map[string]string{"specifier": "npm:@scope/real@^2", "declared_name": "declared"}},
		},
		{
			name: "alias with underscore name", version: "some_pkg@1.0.0",
			want: DependencyReference{Raw: "declared@some_pkg@1.0.0", Name: "some_pkg", Version: "1.0.0", SourceGroup: "dependencies", Attributes: map[string]string{"declared_name": "declared"}},
		},
		{
			name: "v5 alias", version: "/react/18.2.0",
			want: DependencyReference{Raw: "declared@/react/18.2.0", Name: "react", Version: "18.2.0", SourceGroup: "dependencies", Attributes: map[string]string{"declared_name": "declared"}},
		},
		{
			name: "v5 scoped alias and legacy peers", version: "/@scope/real/2.3.4_react@18.2.0",
			want: DependencyReference{Raw: "declared@/@scope/real/2.3.4_react@18.2.0", Name: "@scope/real", Version: "2.3.4", SourceGroup: "dependencies", Attributes: map[string]string{"declared_name": "declared", "peer_context": "_react@18.2.0"}},
		},
		{
			name: "v5 alias with underscore name", version: "/some_pkg/1.0.0_peer@2.0.0",
			want: DependencyReference{Raw: "declared@/some_pkg/1.0.0_peer@2.0.0", Name: "some_pkg", Version: "1.0.0", SourceGroup: "dependencies", Attributes: map[string]string{"declared_name": "declared", "peer_context": "_peer@2.0.0"}},
		},
		{
			name: "v6 alias", version: "/react@18.2.0",
			want: DependencyReference{Raw: "declared@/react@18.2.0", Name: "react", Version: "18.2.0", SourceGroup: "dependencies", Attributes: map[string]string{"declared_name": "declared"}},
		},
		{
			name: "v6 scoped alias and peers", version: "/@scope/real@2.3.4(react@18.2.0)",
			want: DependencyReference{Raw: "declared@/@scope/real@2.3.4(react@18.2.0)", Name: "@scope/real", Version: "2.3.4", SourceGroup: "dependencies", Attributes: map[string]string{"declared_name": "declared", "peer_context": "(react@18.2.0)"}},
		},
		{
			name: "link", version: "link:packages/local",
			want: DependencyReference{Raw: "declared@link:packages/local", Name: "declared", SourceGroup: "dependencies", OriginKind: OriginPath, Attributes: map[string]string{"protocol": "link", "path": "packages/local"}},
		},
		{
			name: "file", version: "file:../local.tgz", specifier: "file:../local.tgz",
			want: DependencyReference{Raw: "declared@file:../local.tgz", Name: "declared", SourceGroup: "dependencies", OriginKind: OriginPath, Attributes: map[string]string{"specifier": "file:../local.tgz", "protocol": "file", "path": "../local.tgz"}},
		},
		{
			name: "path parentheses", version: "link:../local(test)_package",
			want: DependencyReference{Raw: "declared@link:../local(test)_package", Name: "declared", SourceGroup: "dependencies", OriginKind: OriginPath, Attributes: map[string]string{"protocol": "link", "path": "../local(test)_package"}},
		},
		{
			name: "workspace link", version: "link:packages/local", specifier: "workspace:*",
			want: DependencyReference{Raw: "declared@link:packages/local", Name: "declared", SourceGroup: "dependencies", OriginKind: OriginWorkspace, Attributes: map[string]string{"specifier": "workspace:*", "protocol": "link", "path": "packages/local"}},
		},
		{
			name: "unrelated protocol", version: "git+ssh://git@github.com/org/repo.git#commit",
			want: DependencyReference{Raw: "declared@git+ssh://git@github.com/org/repo.git#commit", Name: "declared", Version: "git+ssh://git@github.com/org/repo.git#commit", SourceGroup: "dependencies"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			content := "lockfileVersion: '9.0'\nimporters:\n  .:\n    dependencies:\n      declared:\n        version: '" + test.version + "'\n"
			if test.specifier != "" {
				content += "        specifier: '" + test.specifier + "'\n"
			}
			result, err := (pnpmLockParser{}).Analyze("pnpm-lock.yaml", []byte(content))
			if err != nil {
				t.Fatalf("Analyze failed: %v", err)
			}
			if !equalDependencies(result.Dependencies, []DependencyReference{test.want}) {
				t.Fatalf("unexpected dependencies: got %+v want %+v", result.Dependencies, []DependencyReference{test.want})
			}
		})
	}
}

func TestPNPMLockDeduplicatesLegacyPackageIDs(t *testing.T) {
	tests := []struct {
		name      string
		version   string
		packageID string
	}{
		{name: "v5", version: "/@scope/real/2.3.4_react@18.2.0", packageID: "/@scope/real/2.3.4_react@18.2.0"},
		{name: "v6", version: "/@scope/real@2.3.4(react@18.2.0)", packageID: "/@scope/real@2.3.4(react@18.2.0)"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			schema, reactKey := "6.0", "/react@18.2.0"
			if test.name == "v5" {
				schema, reactKey = "5.4", "/react/18.2.0"
			}
			content := "lockfileVersion: '" + schema + "'\ndependencies:\n  alias: '" + test.version + "'\npackages:\n  '" + test.packageID + "': {}\n  '" + reactKey + "': {}\n"
			result, err := (pnpmLockParser{}).Analyze("pnpm-lock.yaml", []byte(content))
			if err != nil {
				t.Fatalf("Analyze failed: %v", err)
			}
			want := []DependencyReference{
				{Raw: "alias@" + test.version, Name: "@scope/real", Version: "2.3.4", SourceGroup: "dependencies", Attributes: map[string]string{"declared_name": "alias", "peer_context": "(react@18.2.0)"}},
				{Raw: "react@18.2.0", Name: "react", Version: "18.2.0"},
			}
			if test.name == "v5" {
				want[0].Attributes["peer_context"] = "_react@18.2.0"
			}
			if !equalDependencies(result.Dependencies, want) {
				t.Fatalf("unexpected dependencies: got %+v want %+v", result.Dependencies, want)
			}
		})
	}
}
