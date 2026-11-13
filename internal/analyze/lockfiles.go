package analyze

import (
	"path"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
)

// Relationships describe candidate resolutions, not which package manager a build uses.
func mapLockfiles(sources []DependencySourceResult, inputs []policyInput, discovered map[string]struct{}) {
	ctx := buildEvaluationContext(sources, inputs, discovered)
	javascript := make(map[string]javascriptManifest, len(ctx.javascript))
	javascriptSpaces := []workspaceDefinition{}
	for _, manifest := range ctx.javascript {
		javascript[manifest.root] = manifest
		if len(manifest.workspaces) > 0 {
			javascriptSpaces = append(javascriptSpaces, workspaceDefinition{root: manifest.root, patterns: manifest.workspaces})
		}
	}
	for _, source := range sources {
		switch source.Detector {
		case "js":
			root := javascriptCatalogRoot(source.Path)
			if _, exists := javascript[root]; !exists {
				javascript[root] = javascriptManifest{root: root, managerInvalid: true}
			}
		case "js-pnpm-workspace":
			space, err := readPNPMWorkspace(source.content, source.Path)
			if err != nil {
				space = workspaceDefinition{root: javascriptCatalogRoot(source.Path), manager: "pnpm"}
			}
			javascriptSpaces = append(javascriptSpaces, space)
		case "js-yarnrc":
			javascriptSpaces = append(javascriptSpaces, workspaceDefinition{root: javascriptCatalogRoot(source.Path)})
		}
	}
	uv, cargo := lockfileTOMLWorkspaces(sources, inputs)
	for i := range sources {
		source := &sources[i]
		if source.Form != FormManifest && source.Form != FormBuildDefinition {
			continue
		}
		root := javascriptCatalogRoot(source.Path)
		owners := map[string]string{}
		switch path.Base(source.Path) {
		case "package.json":
			for _, name := range []string{"package-lock.json", "npm-shrinkwrap.json", "pnpm-lock.yaml", "yarn.lock", "bun.lock", "bun.lockb"} {
				owners[name] = lockfileJavaScriptOwner(root, name, javascript, javascriptSpaces)
			}
		case "pyproject.toml":
			owners = map[string]string{"uv.lock": lockfileTOMLOwner(root, uv), "poetry.lock": root, "pdm.lock": root}
		case "Pipfile":
			owners["Pipfile.lock"] = root
		case "Cargo.toml":
			owners["Cargo.lock"] = lockfileTOMLOwner(root, cargo)
		case "composer.json":
			disabled := false
			for _, manifest := range ctx.composer {
				if manifest.path == source.Path {
					disabled = manifest.lockDisabled
				}
			}
			if !disabled {
				owners["composer.lock"] = root
			}
		case "build.gradle", "build.gradle.kts":
			owners["gradle.lockfile"] = root
		case "Gemfile":
			owners["Gemfile.lock"] = root
		default:
			switch path.Ext(source.Path) {
			case ".csproj", ".fsproj", ".vbproj":
				owners["packages.lock.json"] = root
			default:
				continue
			}
		}
		source.Lockfiles = []string{}
		for name, owner := range owners {
			for _, dir := range []string{root, owner} {
				candidate := joinRoot(dir, name)
				if _, exists := discovered[candidate]; exists && !slices.Contains(source.Lockfiles, candidate) {
					source.Lockfiles = append(source.Lockfiles, candidate)
				}
			}
		}
		slices.Sort(source.Lockfiles)
	}
}

func lockfileJavaScriptOwner(root, lockfile string, manifests map[string]javascriptManifest, spaces []workspaceDefinition) string {
	manifest, exists := manifests[root]
	if !exists || len(manifest.workspaces) > 0 || manifest.managerInvalid {
		return root
	}
	for ancestor := root; ; ancestor = javascriptCatalogRoot(ancestor) {
		var patterns []string
		configured := false
		for _, space := range spaces {
			if space.root != ancestor {
				continue
			}
			// pnpm membership comes from pnpm-workspace.yaml. Other managers use package.json.
			if (lockfile == "pnpm-lock.yaml") != (space.manager == "pnpm") {
				continue
			}
			configured = true
			patterns = append(patterns, space.patterns...)
		}
		if configured {
			if ancestor == root || !lockfileWorkspaceMatches(javascriptCatalogRelative(ancestor, root), patterns) {
				return root
			}
			for intermediate := javascriptCatalogRoot(root); intermediate != ancestor; intermediate = javascriptCatalogRoot(intermediate) {
				if nested, exists := manifests[intermediate]; exists && (len(nested.workspaces) > 0 || nested.managerInvalid ||
					!lockfileWorkspaceMatches(javascriptCatalogRelative(ancestor, intermediate), patterns)) {
					return root
				}
			}
			return ancestor
		}
		if ancestor != root {
			if nested, exists := manifests[ancestor]; exists && (len(nested.workspaces) > 0 || nested.managerInvalid) {
				return root
			}
		}
		if ancestor == "" {
			return root
		}
	}
}

type lockfileTOMLProject struct {
	workspace *struct {
		Members []string `toml:"members"`
		Exclude []string `toml:"exclude"`
	}
	independent bool
}

func lockfileTOMLWorkspaces(sources []DependencySourceResult, inputs []policyInput) (map[string]lockfileTOMLProject, map[string]lockfileTOMLProject) {
	uv, cargo := map[string]lockfileTOMLProject{}, map[string]lockfileTOMLProject{}
	for _, input := range inputs {
		var document struct {
			Tool struct {
				UV struct {
					Workspace *struct {
						Members []string `toml:"members"`
						Exclude []string `toml:"exclude"`
					} `toml:"workspace"`
					Managed *bool `toml:"managed"`
				} `toml:"uv"`
			} `toml:"tool"`
		}
		err := input.readError
		if err == nil {
			_, err = toml.Decode(string(input.content), &document)
		}
		uv[javascriptCatalogRoot(input.path)] = lockfileTOMLProject{workspace: document.Tool.UV.Workspace,
			independent: err != nil || (document.Tool.UV.Managed != nil && !*document.Tool.UV.Managed)}
	}
	for _, source := range sources {
		if path.Base(source.Path) != "Cargo.toml" {
			continue
		}
		var document struct {
			Workspace *struct {
				Members []string `toml:"members"`
				Exclude []string `toml:"exclude"`
			} `toml:"workspace"`
		}
		_, err := toml.Decode(string(source.content), &document)
		cargo[javascriptCatalogRoot(source.Path)] = lockfileTOMLProject{workspace: document.Workspace, independent: err != nil}
	}
	return uv, cargo
}

func lockfileTOMLOwner(root string, projects map[string]lockfileTOMLProject) string {
	for ancestor := root; ; ancestor = javascriptCatalogRoot(ancestor) {
		project := projects[ancestor]
		if project.independent || (ancestor == root && project.workspace != nil) {
			return root
		}
		if workspace := project.workspace; workspace != nil {
			relative := javascriptCatalogRelative(ancestor, root)
			if !lockfileWorkspaceMatches(relative, workspace.Members) {
				return root
			}
			for member := root; member != ancestor; member = javascriptCatalogRoot(member) {
				if _, exists := projects[member]; !exists {
					continue
				}
				for _, excluded := range workspace.Exclude {
					if pathGlobMatches(strings.TrimPrefix(excluded, "!"), javascriptCatalogRelative(ancestor, member)) {
						return root
					}
				}
			}
			return ancestor
		}
		if ancestor == "" {
			return root
		}
	}
}

func lockfileWorkspaceMatches(relative string, patterns []string) bool {
	included := false
	for _, pattern := range patterns {
		if strings.HasPrefix(pattern, "!") {
			if pathGlobMatches(strings.TrimPrefix(pattern, "!"), relative) {
				return false
			}
		} else if pathGlobMatches(pattern, relative) {
			included = true
		}
	}
	return included
}
