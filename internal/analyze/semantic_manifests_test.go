package analyze

import (
	"path/filepath"
	"testing"
)

func TestMavenPOMSemanticFixture(t *testing.T) {
	result, err := Scan(filepath.Join("..", "..", "testdata", "java", "maven-pom-semantic"), nil, mustLoadDefaultRules(t))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	dependencies := dependenciesForSource(t, result, "pom.xml")
	if len(dependencies) != 3 {
		t.Fatalf("dependencies = %+v, want three", dependencies)
	}
	assertDependency(t, dependencies[0], "org.junit.jupiter:junit-jupiter", "[5.11.0]", "dependencies", RelationshipDirect, ScopeTest)
	assertDependency(t, dependencies[1], "org.slf4j:slf4j-api", "[2.0,3.0)", "dependencies", RelationshipDirect, ScopeRuntime)
	if dependencies[1].Raw != "org.slf4j:slf4j-api:${slf4j.range}" || dependencies[1].VERS != "vers:maven/>=2.0|<3.0" {
		t.Fatalf("property-backed Maven dependency = %+v", dependencies[1])
	}
	assertDependency(t, dependencies[2], "org.springframework.boot:spring-boot-dependencies", "[3.3.2]", "dependencyManagement", RelationshipInconclusive, ScopeBuild)
	if dependencies[2].Attributes["maven_scope"] != "import" || dependencies[2].Attributes["type"] != "pom" {
		t.Fatalf("managed Maven attributes = %+v", dependencies[2])
	}
}

func TestMavenPOMUnresolvedPropertyIsPartial(t *testing.T) {
	result, err := Scan(filepath.Join("..", "..", "testdata", "java", "maven-pom-unresolved"), nil, mustLoadDefaultRules(t))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	source := sourceForPath(t, result, "pom.xml")
	if source.Analysis.Extraction != ExtractionPartial || len(source.Dependencies) != 1 ||
		source.Dependencies[0].VersionConstraint != "${revision}" {
		t.Fatalf("source = %+v", source)
	}
	if len(source.Diagnostics) != 1 || source.Diagnostics[0].Code != incompleteExtractionCode {
		t.Fatalf("diagnostics = %+v", source.Diagnostics)
	}
}

func TestCargoManifestSemanticFixture(t *testing.T) {
	result, err := Scan(filepath.Join("..", "..", "testdata", "rust", "cargo-manifest-semantic"), nil, mustLoadDefaultRules(t))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	dependencies := dependenciesForSource(t, result, "Cargo.toml")
	if len(dependencies) != 5 {
		t.Fatalf("dependencies = %+v, want five", dependencies)
	}
	renamed := dependencyByName(t, dependencies, "serde_json")
	if renamed.Attributes["declared_name"] != "json" || renamed.OriginKind != OriginRegistry ||
		renamed.Relationship != RelationshipDirect || renamed.Scope != ScopeOptional ||
		renamed.VERS != "vers:cargo/>=1.0.0|<2.0.0" {
		t.Fatalf("renamed Cargo dependency = %+v", renamed)
	}
	target := dependencyByName(t, dependencies, "cc")
	if target.SourceGroup != "target.cfg(unix).build-dependencies" || target.Scope != ScopeBuild ||
		target.Attributes["target"] != "cfg(unix)" {
		t.Fatalf("target Cargo dependency = %+v", target)
	}
	workspace := dependencyByName(t, dependencies, "anyhow")
	if workspace.SourceGroup != "workspace.dependencies" || workspace.Relationship != RelationshipInconclusive {
		t.Fatalf("workspace Cargo dependency = %+v", workspace)
	}
}

func TestCargoManifestNormalizesGitPathAndWorkspaceOrigins(t *testing.T) {
	parser, err := newCargoManifestParser(cargoManifestMatcherConfig{})
	if err != nil {
		t.Fatalf("newCargoManifestParser: %v", err)
	}
	result, err := parser.Analyze("Cargo.toml", []byte(`
[dependencies]
git-client = { package = "client", git = "https://example.test/client.git", tag = "v2" }
local-client = { path = "../client" }
shared = { workspace = true }
`))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	gitDependency := dependencyByName(t, result.Dependencies, "client")
	if gitDependency.OriginKind != OriginGit || gitDependency.Attributes["declared_name"] != "git-client" ||
		gitDependency.Attributes["source_url"] != "https://example.test/client.git" ||
		gitDependency.Attributes["source_ref"] != "v2" {
		t.Fatalf("Git dependency = %+v", gitDependency)
	}
	pathDependency := dependencyByName(t, result.Dependencies, "local-client")
	if pathDependency.OriginKind != OriginPath || pathDependency.Attributes["path"] != "../client" {
		t.Fatalf("path dependency = %+v", pathDependency)
	}
	workspaceDependency := dependencyByName(t, result.Dependencies, "shared")
	if workspaceDependency.OriginKind != OriginWorkspace || workspaceDependency.Relationship != RelationshipDirect {
		t.Fatalf("workspace dependency = %+v", workspaceDependency)
	}
}

func TestComposerManifestSemanticFixtureExcludesPlatformPackages(t *testing.T) {
	result, err := Scan(filepath.Join("..", "..", "testdata", "php", "composer-json-semantic"), nil, mustLoadDefaultRules(t))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	dependencies := dependenciesForSource(t, result, "composer.json")
	if len(dependencies) != 2 {
		t.Fatalf("dependencies = %+v, want two", dependencies)
	}
	assertDependency(t, dependencies[0], "monolog/monolog", "^3.0", "require", RelationshipDirect, ScopeRuntime)
	assertDependency(t, dependencies[1], "phpunit/phpunit", "^11.0", "require-dev", RelationshipDirect, ScopeDevelopment)
}

func TestDotnetSemanticFixtures(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	project, err := Scan(filepath.Join("..", "..", "testdata", "dotnet", "project-semantic"), nil, ruleset)
	if err != nil {
		t.Fatalf("Scan project: %v", err)
	}
	if len(project.Sources) != 3 {
		t.Fatalf("project sources = %+v, want csproj, fsproj, and vbproj", project.Sources)
	}
	csproj := sourceForPath(t, project, "app.csproj")
	if csproj.Detector != "dotnet-csproj" || len(csproj.Dependencies) != 2 {
		t.Fatalf("csproj = %+v", csproj)
	}
	if dependencyByName(t, csproj.Dependencies, "coverlet.collector").Scope != ScopeBuild {
		t.Fatalf("coverlet scope = %+v", csproj.Dependencies)
	}
	if sourceForPath(t, project, "app.fsproj").Detector != "dotnet-fsproj" ||
		sourceForPath(t, project, "app.vbproj").Detector != "dotnet-vbproj" {
		t.Fatalf("project detector coverage = %+v", project.Sources)
	}

	central, err := Scan(filepath.Join("..", "..", "testdata", "dotnet", "central-packages-semantic"), nil, ruleset)
	if err != nil {
		t.Fatalf("Scan central packages: %v", err)
	}
	centralDependency := dependenciesForSource(t, central, "Directory.Packages.props")[0]
	if centralDependency.VersionConstraint != "[8.0.0,9.0.0)" || centralDependency.Relationship != RelationshipInconclusive ||
		centralDependency.Raw != "Microsoft.Extensions.Logging@$(LoggingVersion)" {
		t.Fatalf("central dependency = %+v", centralDependency)
	}

	legacy, err := Scan(filepath.Join("..", "..", "testdata", "dotnet", "packages-config-semantic"), nil, ruleset)
	if err != nil {
		t.Fatalf("Scan packages.config: %v", err)
	}
	legacyDependencies := dependenciesForSource(t, legacy, "packages.config")
	if len(legacyDependencies) != 2 || dependencyByName(t, legacyDependencies, "Newtonsoft.Json").Version != "13.0.3" ||
		dependencyByName(t, legacyDependencies, "xunit.runner.visualstudio").Scope != ScopeDevelopment {
		t.Fatalf("legacy dependencies = %+v", legacyDependencies)
	}
}

func TestDotnetUnresolvedPropertyIsPartial(t *testing.T) {
	result, err := Scan(filepath.Join("..", "..", "testdata", "dotnet", "project-unresolved"), nil, mustLoadDefaultRules(t))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	source := sourceForPath(t, result, "app.csproj")
	if source.Analysis.Extraction != ExtractionPartial || source.Dependencies[0].VersionConstraint != "$(ClientVersion)" ||
		len(source.Diagnostics) != 1 {
		t.Fatalf("source = %+v", source)
	}
}

func TestDotnetProjectSupportsChildVersionsUpdatesAndConditions(t *testing.T) {
	parser, err := newDotnetProjectParser(dotnetProjectMatcherConfig{})
	if err != nil {
		t.Fatalf("newDotnetProjectParser: %v", err)
	}
	result, err := parser.Analyze("app.csproj", []byte(`
<Project>
  <PropertyGroup><ClientVersion>[2.0.0]</ClientVersion></PropertyGroup>
  <ItemGroup Condition="'$(TargetFramework)' == 'net8.0'">
    <PackageReference Include="Contoso.Client">
      <Version>$(ClientVersion)</Version>
    </PackageReference>
    <PackageReference Update="Build.Tasks" Version="[1.0.0]" />
  </ItemGroup>
</Project>`))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	client := dependencyByName(t, result.Dependencies, "Contoso.Client")
	if client.VersionConstraint != "[2.0.0]" || client.Relationship != RelationshipDirect ||
		client.Attributes["condition"] != "'$(TargetFramework)' == 'net8.0'" {
		t.Fatalf("client dependency = %+v", client)
	}
	update := dependencyByName(t, result.Dependencies, "Build.Tasks")
	if update.Relationship != RelationshipInconclusive || update.Attributes["item_operation"] != "Update" {
		t.Fatalf("updated dependency = %+v", update)
	}
}

func assertDependency(t *testing.T, dependency DependencyReference, name, constraint, group string, relationship Relationship, scope DependencyScope) {
	t.Helper()
	if dependency.Name != name || dependency.VersionConstraint != constraint || dependency.SourceGroup != group ||
		dependency.Relationship != relationship || dependency.Scope != scope {
		t.Fatalf("dependency = %+v, want name=%q constraint=%q group=%q relationship=%q scope=%q",
			dependency, name, constraint, group, relationship, scope)
	}
}

func dependencyByName(t *testing.T, dependencies []DependencyReference, name string) DependencyReference {
	t.Helper()
	for _, dependency := range dependencies {
		if dependency.Name == name {
			return dependency
		}
	}
	t.Fatalf("dependency %q not found in %+v", name, dependencies)
	return DependencyReference{}
}

func sourceForPath(t *testing.T, result ScanResult, path string) DependencySourceResult {
	t.Helper()
	for _, source := range result.Sources {
		if source.Path == path {
			return source
		}
	}
	t.Fatalf("source %q not found in %+v", path, result.Sources)
	return DependencySourceResult{}
}
