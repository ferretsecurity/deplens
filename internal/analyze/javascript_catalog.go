package analyze

import (
	"fmt"
	"path"
	"strings"

	"go.yaml.in/yaml/v3"
)

type javascriptCatalogConfig struct {
	path     string
	root     string
	manager  string
	patterns []string
	catalogs map[string]map[string]string
	err      error
}

func resolveJavaScriptCatalogs(sources []DependencySourceResult) {
	manifests := make(map[string]javascriptManifest)
	configs := make(map[string][]javascriptCatalogConfig)
	for _, source := range sources {
		if source.Detector == "js" {
			if manifest, err := readJavaScriptManifest(source.content, source); err == nil {
				manifests[manifest.root] = manifest
			}
		}
	}
	for _, source := range sources {
		if source.Detector != "js-pnpm-workspace" && source.Detector != "js-yarnrc" {
			continue
		}
		config := javascriptCatalogConfig{path: source.Path, root: javascriptCatalogRoot(source.Path), manager: "pnpm"}
		if source.Detector == "js-yarnrc" {
			config.manager = "yarn"
			config.patterns = manifests[config.root].workspaces
		}
		config.catalogs, config.err = readJavaScriptCatalogs(source.content, config.manager)
		if config.manager == "pnpm" && config.err == nil {
			workspace, err := readPNPMWorkspace(source.content, source.Path)
			config.patterns, config.err = workspace.patterns, err
		}
		configs[config.root] = append(configs[config.root], config)
	}
	for index := range sources {
		source := &sources[index]
		if source.Detector != "js" {
			continue
		}
		for index := range source.Dependencies {
			dependency := &source.Dependencies[index]
			specifier := strings.TrimPrefix(dependency.Raw, dependency.Name+"@")
			if !strings.HasPrefix(specifier, "catalog:") {
				continue
			}
			catalogName := strings.TrimPrefix(specifier, "catalog:")
			dependency.Attributes = map[string]string{"catalog_name": catalogName}
			if catalogName == "" {
				dependency.Attributes["catalog_name"] = "default"
			}
			config, err := javascriptCatalogOwner(javascriptCatalogRoot(source.Path), configs, manifests)
			if config != nil {
				dependency.Attributes["catalog_source"] = config.path
				if config.manager == "pnpm" && catalogName == "" {
					catalogName = "default"
				}
			}
			if err == nil {
				target, ok := config.catalogs[catalogName][dependency.Name]
				if !ok {
					err = fmt.Errorf("catalog %q has no entry for %q", dependency.Attributes["catalog_name"], dependency.Name)
				} else {
					dependency.Attributes["catalog_specifier"] = target
					resolved := packageJSONDependency(dependency.Name, target, dependency.SourceGroup, dependency.Scope)
					if strings.HasPrefix(target, "catalog:") || resolved.OriginKind == "" {
						err = fmt.Errorf("catalog target %q cannot be normalized", target)
					} else {
						resolved.Raw = dependency.Raw
						resolved.PackageType = dependency.PackageType
						resolved.Relationship = dependency.Relationship
						if resolved.Attributes == nil {
							resolved.Attributes = make(map[string]string)
						}
						for key, value := range dependency.Attributes {
							resolved.Attributes[key] = value
						}
						*dependency = resolved
						applyDependencyVERS(source.Dependencies[index : index+1])
					}
				}
			}
			if err != nil {
				source.Analysis.Extraction = ExtractionPartial
				source.Diagnostics = append(source.Diagnostics, Diagnostic{
					Severity: DiagnosticWarning, Code: "javascript-catalog-unresolved",
					Message: fmt.Sprintf("%s.%s: cannot resolve %q: %v", dependency.SourceGroup, dependency.Name, specifier, err),
				})
			}
		}
	}
}

func javascriptCatalogOwner(root string, configs map[string][]javascriptCatalogConfig, manifests map[string]javascriptManifest) (*javascriptCatalogConfig, error) {
	owner := root
	for {
		if candidates := configs[owner]; len(candidates) > 0 {
			if len(candidates) != 1 {
				return nil, fmt.Errorf("multiple catalog configurations at %q", projectSubject(owner).ProjectRoot)
			}
			config := &candidates[0]
			if config.err != nil {
				return config, fmt.Errorf("parse %s: %w", config.path, config.err)
			}
			manifest := manifests[root]
			ownerManifest := manifests[owner]
			if manifest.managerInvalid || ownerManifest.managerInvalid ||
				(manifest.manager != "" && manifest.manager != config.manager) ||
				(ownerManifest.manager != "" && ownerManifest.manager != config.manager) {
				return config, fmt.Errorf("package manager conflicts with %s", config.path)
			}
			if owner != root {
				if len(manifest.workspaces) > 0 || !workspaceMatches(javascriptCatalogRelative(owner, root), config.patterns) {
					return config, fmt.Errorf("manifest is not a member of %s", config.path)
				}
				for ancestor := javascriptCatalogRoot(root); ancestor != owner; ancestor = javascriptCatalogRoot(ancestor) {
					if nested, ok := manifests[ancestor]; ok && (len(nested.workspaces) > 0 || nested.manager != "" || nested.managerInvalid ||
						!workspaceMatches(javascriptCatalogRelative(owner, ancestor), config.patterns)) {
						return config, fmt.Errorf("nested project at %q separates the manifest from %s", ancestor, config.path)
					}
				}
			}
			return config, nil
		}
		if owner == "" {
			return nil, fmt.Errorf("no catalog configuration owns this manifest")
		}
		owner = javascriptCatalogRoot(owner)
	}
}

func javascriptCatalogRoot(sourcePath string) string {
	root := path.Dir(sourcePath)
	if root == "." {
		return ""
	}
	return root
}

func javascriptCatalogRelative(root, child string) string {
	return strings.TrimPrefix(strings.TrimPrefix(child, root), "/")
}

func readJavaScriptCatalogs(content []byte, manager string) (map[string]map[string]string, error) {
	var document yaml.Node
	if err := yaml.Unmarshal(content, &document); err != nil {
		return nil, err
	}
	if len(document.Content) != 1 {
		return nil, fmt.Errorf("expected a configuration mapping")
	}
	root, err := javascriptCatalogMapping(document.Content[0])
	if err != nil {
		return nil, err
	}
	catalogs := make(map[string]map[string]string)
	if node, ok := root["catalog"]; ok {
		name := ""
		if manager == "pnpm" {
			name = "default"
		}
		catalogs[name], err = readJavaScriptCatalogEntries(node)
		if err != nil {
			return nil, fmt.Errorf("catalog: %w", err)
		}
	}
	if node, ok := root["catalogs"]; ok {
		named, err := javascriptCatalogMapping(node)
		if err != nil {
			return nil, fmt.Errorf("catalogs: %w", err)
		}
		for _, name := range sortedStringKeys(named) {
			if name == "" {
				return nil, fmt.Errorf("catalog name must not be empty")
			}
			if _, exists := catalogs[name]; exists {
				return nil, fmt.Errorf("catalog %q is defined more than once", name)
			}
			catalogs[name], err = readJavaScriptCatalogEntries(named[name])
			if err != nil {
				return nil, fmt.Errorf("catalogs.%s: %w", name, err)
			}
		}
	}
	return catalogs, nil
}

func readJavaScriptCatalogEntries(node *yaml.Node) (map[string]string, error) {
	entries, err := javascriptCatalogMapping(node)
	if err != nil {
		return nil, err
	}
	result := make(map[string]string, len(entries))
	for _, name := range sortedStringKeys(entries) {
		value := javascriptCatalogValue(entries[name])
		if name == "" || value == nil || value.Kind != yaml.ScalarNode || value.Tag != "!!str" || strings.TrimSpace(value.Value) == "" {
			return nil, fmt.Errorf("%q: expected a non-empty string dependency specifier", name)
		}
		result[name] = value.Value
	}
	return result, nil
}

func javascriptCatalogMapping(node *yaml.Node) (map[string]*yaml.Node, error) {
	node = javascriptCatalogValue(node)
	if node == nil || node.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("expected a mapping")
	}
	result := make(map[string]*yaml.Node, len(node.Content)/2)
	for index := 0; index < len(node.Content); index += 2 {
		key := node.Content[index]
		if key.Kind != yaml.ScalarNode || key.Tag != "!!str" {
			return nil, fmt.Errorf("expected a string mapping key")
		}
		if _, exists := result[key.Value]; exists {
			return nil, fmt.Errorf("duplicate mapping key %q", key.Value)
		}
		result[key.Value] = node.Content[index+1]
	}
	return result, nil
}

func javascriptCatalogValue(node *yaml.Node) *yaml.Node {
	seen := make(map[*yaml.Node]bool)
	for node != nil && node.Kind == yaml.AliasNode {
		if seen[node] {
			return nil
		}
		seen[node] = true
		node = node.Alias
	}
	return node
}
