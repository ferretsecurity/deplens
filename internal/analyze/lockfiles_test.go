package analyze

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestScanMapsCompetingAndWorkspaceLockfiles(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"package.json":            `{"private":true,"workspaces":["apps/*"]}`,
		"apps/web/package.json":   `{"dependencies":{"widget":"^2.3.0"}}`,
		"standalone/package.json": `{"dependencies":{"widget":"^2.3.0"}}`,
		"package-lock.json":       `{"lockfileVersion":3,"packages":{}}`,
		"yarn.lock":               "# yarn lockfile v1\n",
	}
	for name, content := range files {
		full := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	result, err := Scan(root, nil, mustLoadDefaultRules(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range result.Sources {
		switch source.Path {
		case "package.json", "apps/web/package.json":
			if !slices.Equal(source.Lockfiles, []string{"package-lock.json", "yarn.lock"}) {
				t.Fatalf("%s lockfiles: %v", source.Path, source.Lockfiles)
			}
		case "standalone/package.json":
			if source.Lockfiles == nil || len(source.Lockfiles) != 0 {
				t.Fatalf("unrelated project: %v", source.Lockfiles)
			}
		}
	}
}

func TestScanMapsOnlyExplicitWorkspaceMembersAndLockFamilies(t *testing.T) {
	for _, tc := range []struct {
		name  string
		files map[string]string
		want  map[string][]string
	}{
		{
			name: "javascript exclusions and nested projects",
			files: map[string]string{
				"package.json":                             `{"workspaces":["!apps/excluded", "apps/**"]}`,
				"package-lock.json":                        `{"lockfileVersion":3,"packages":{}}`,
				"pnpm-lock.yaml":                           "lockfileVersion: '9.0'\n",
				"apps/member/package.json":                 `{}`,
				"apps/excluded/package.json":               `{}`,
				"apps/nested/package.json":                 `{"workspaces":["children/*"]}`,
				"apps/nested/children/a/package.json":      `{}`,
				"apps/declared-manager/package.json":       `{"packageManager":"npm@10.0.0"}`,
				"apps/declared-manager/child/package.json": `{}`,
			},
			want: map[string][]string{
				"apps/member/package.json":                 {"package-lock.json"},
				"apps/excluded/package.json":               {},
				"apps/nested/package.json":                 {},
				"apps/nested/children/a/package.json":      {},
				"apps/declared-manager/child/package.json": {"package-lock.json"},
				"apps/declared-manager/package.json":       {"package-lock.json"},
			},
		},
		{
			name: "package manager declaration retains explicit membership",
			files: map[string]string{
				"package.json":             `{"private":true,"packageManager":"npm@12.2.0","workspaces":["apps/*"]}`,
				"apps/member/package.json": `{"name":"member","version":"1.0.0","packageManager":"npm@12.2.0"}`,
				"package-lock.json":        `{"lockfileVersion":3,"packages":{"apps/member":{"name":"member","version":"1.0.0"},"node_modules/member":{"resolved":"apps/member","link":true}}}`,
			},
			want: map[string][]string{
				"apps/member/package.json": {"package-lock.json"},
			},
		},
		{
			name: "pnpm membership requires pnpm configuration",
			files: map[string]string{
				"package.json":               `{"workspaces":["apps/*"]}`,
				"package-lock.json":          `{"lockfileVersion":3,"packages":{}}`,
				"pnpm-workspace.yaml":        "packages:\n  - '!apps/excluded'\n  - 'apps/*'\n",
				"pnpm-lock.yaml":             "lockfileVersion: '9.0'\n",
				"apps/member/package.json":   `{}`,
				"apps/excluded/package.json": `{}`,
			},
			want: map[string][]string{
				"apps/member/package.json":   {"package-lock.json", "pnpm-lock.yaml"},
				"apps/excluded/package.json": {"package-lock.json"},
			},
		},
		{
			name: "uv excludes and separate poetry lock",
			files: map[string]string{
				"pyproject.toml":                        "[tool.uv.workspace]\nmembers = ['libs/**']\nexclude = ['libs/excluded']\n",
				"uv.lock":                               "version = 1\n",
				"poetry.lock":                           "package = []\n",
				"libs/member/pyproject.toml":            "[project]\nname = 'member'\nversion = '1.0.0'\ndependencies = ['widget>=1']\n",
				"libs/excluded/pyproject.toml":          "[project]\nname = 'excluded'\nversion = '1.0.0'\ndependencies = ['widget>=1']\n",
				"libs/excluded/child/pyproject.toml":    "[project]\nname = 'child'\nversion = '1.0.0'\ndependencies = ['widget>=1']\n",
				"libs/nested/pyproject.toml":            "[project]\ndependencies = ['widget>=1']\n[tool.uv.workspace]\nmembers = ['children/*']\n",
				"libs/nested/children/a/pyproject.toml": "[project]\nname = 'nested-child'\nversion = '1.0.0'\ndependencies = ['widget>=1']\n",
			},
			want: map[string][]string{
				"libs/member/pyproject.toml":            {"uv.lock"},
				"libs/excluded/pyproject.toml":          {},
				"libs/excluded/child/pyproject.toml":    {},
				"libs/nested/pyproject.toml":            {},
				"libs/nested/children/a/pyproject.toml": {},
			},
		},
		{
			name: "cargo exclusions and nested workspace",
			files: map[string]string{
				"Cargo.toml":                 "[workspace]\nmembers = ['crates/**']\nexclude = ['crates/excluded']\n",
				"Cargo.lock":                 "version = 3\n",
				"crates/member/Cargo.toml":   "[package]\nname = 'member'\nversion = '1.0.0'\n",
				"crates/excluded/Cargo.toml": "[package]\nname = 'excluded'\nversion = '1.0.0'\n",
				"crates/nested/Cargo.toml":   "[workspace]\n",
			},
			want: map[string][]string{
				"crates/member/Cargo.toml":   {"Cargo.lock"},
				"crates/excluded/Cargo.toml": {},
				"crates/nested/Cargo.toml":   {},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			for name, content := range tc.files {
				full := filepath.Join(root, name)
				if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(full, []byte(content), 0600); err != nil {
					t.Fatal(err)
				}
			}
			result, err := Scan(root, nil, mustLoadDefaultRules(t))
			if err != nil {
				t.Fatal(err)
			}
			checked := map[string]bool{}
			for _, source := range result.Sources {
				if want, exists := tc.want[source.Path]; exists {
					checked[source.Path] = true
					if source.Lockfiles == nil || !slices.Equal(source.Lockfiles, want) {
						t.Errorf("%s lockfiles: got %v want %v", source.Path, source.Lockfiles, want)
					}
				}
			}
			for manifest := range tc.want {
				if !checked[manifest] {
					t.Errorf("manifest not scanned: %s", manifest)
				}
			}
		})
	}
}

func TestScanMapsLocalGradleAndNuGetLockfiles(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"java/build.gradle":         `dependencies { implementation 'org:widget:2.3.0' }`,
		"kotlin/build.gradle.kts":   `dependencies { implementation("org:widget:2.3.0") }`,
		"java/gradle.lockfile":      "org:widget:2.3.5=runtimeClasspath\n",
		"kotlin/gradle.lockfile":    "org:widget:2.3.5=runtimeClasspath\n",
		"dotnet/app.csproj":         `<Project><ItemGroup><PackageReference Include="Widget" Version="2.3.0" /></ItemGroup></Project>`,
		"dotnet/app.fsproj":         `<Project><ItemGroup><PackageReference Include="Widget" Version="2.3.0" /></ItemGroup></Project>`,
		"dotnet/app.vbproj":         `<Project><ItemGroup><PackageReference Include="Widget" Version="2.3.0" /></ItemGroup></Project>`,
		"dotnet/packages.lock.json": `{"version":1,"dependencies":{"net8.0":{"Widget":{"type":"Direct","requested":"[2.3.0, )","resolved":"2.3.5"}}}}`,
	}
	for name, content := range files {
		full := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	result, err := Scan(root, nil, mustLoadDefaultRules(t))
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, source := range result.Sources {
		var want string
		switch filepath.Ext(source.Path) {
		case ".gradle", ".kts":
			want = filepath.ToSlash(filepath.Join(filepath.Dir(source.Path), "gradle.lockfile"))
		case ".csproj", ".fsproj", ".vbproj":
			want = "dotnet/packages.lock.json"
		default:
			continue
		}
		seen++
		if !slices.Equal(source.Lockfiles, []string{want}) {
			t.Errorf("%s lockfiles: got %v want %s", source.Path, source.Lockfiles, want)
		}
	}
	if seen != 5 {
		t.Errorf("build definitions: got %d want 5", seen)
	}
}
