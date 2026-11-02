package analyze

import (
	"reflect"
	"strings"
	"testing"
)

func TestMavenPOMProfilePropertiesAreLocal(t *testing.T) {
	result, err := (mavenPOMParser{}).Analyze("pom.xml", []byte(`<project>
  <groupId>org.example</groupId>
  <properties><revision>1.0</revision><shared>${revision}</shared></properties>
  <dependencies><dependency><groupId>${project.groupId}</groupId><artifactId>root</artifactId><version>${shared}</version></dependency></dependencies>
  <profiles>
    <profile>
      <id>first</id>
      <properties><revision>2.0</revision><first.only>3.0</first.only><project.groupId>org.override</project.groupId></properties>
      <dependencies><dependency><groupId>${project.groupId}</groupId><artifactId>first</artifactId><version>${shared}</version></dependency></dependencies>
    </profile>
    <profile>
      <id>second</id>
      <dependencies><dependency><groupId>org.example</groupId><artifactId>second</artifactId><version>${shared}</version></dependency></dependencies>
    </profile>
    <profile>
      <id>third</id>
      <dependencies><dependency><groupId>org.example</groupId><artifactId>third</artifactId><version>${first.only}</version></dependency></dependencies>
    </profile>
  </profiles>
</project>`))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionPartial}) || len(result.Dependencies) != 4 {
		t.Fatalf("result = %+v", result)
	}
	assertDependency(t, dependencyByName(t, result.Dependencies, "org.example:root"), "org.example:root", "[1.0]", "dependencies", RelationshipDirect, ScopeRuntime)
	assertDependency(t, dependencyByName(t, result.Dependencies, "org.example:first"), "org.example:first", "[2.0]", "profiles.first.dependencies", RelationshipDirect, ScopeRuntime)
	assertDependency(t, dependencyByName(t, result.Dependencies, "org.example:second"), "org.example:second", "[1.0]", "profiles.second.dependencies", RelationshipDirect, ScopeRuntime)
	assertDependency(t, dependencyByName(t, result.Dependencies, "org.example:third"), "org.example:third", "${first.only}", "profiles.third.dependencies", RelationshipDirect, ScopeRuntime)
	root := dependencyByName(t, result.Dependencies, "org.example:root")
	if len(root.Attributes) != 0 {
		t.Fatalf("root attributes = %+v", root.Attributes)
	}
	if len(result.Diagnostics) != 1 || !strings.Contains(result.Diagnostics[0].Message, "profiles.third.dependencies") {
		t.Fatalf("diagnostics = %+v", result.Diagnostics)
	}
}

func TestMavenPOMProfileActivationIsPreserved(t *testing.T) {
	activation := `<activeByDefault>true</activeByDefault><jdk>[17,)</jdk><os><family>unix</family></os><property><name>benchmark</name><value>enabled</value></property><file><exists>benchmark.flag</exists></file>`
	result, err := (mavenPOMParser{}).Analyze("pom.xml", []byte(`<project><profiles>
  <profile><id>conditional</id><activation>`+activation+`</activation>
    <dependencies><dependency><groupId>org.example</groupId><artifactId>client</artifactId><version>1.0</version><scope>test</scope><classifier>tests</classifier></dependency></dependencies>
    <dependencyManagement><dependencies><dependency><groupId>org.example</groupId><artifactId>bom</artifactId><version>2.0</version><type>pom</type><scope>import</scope></dependency></dependencies></dependencyManagement>
  </profile>
  <profile><id>manual</id><dependencies><dependency><groupId>org.example</groupId><artifactId>manual</artifactId></dependency></dependencies></profile>
</profiles></project>`))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) || len(result.Dependencies) != 3 {
		t.Fatalf("result = %+v", result)
	}
	client := dependencyByName(t, result.Dependencies, "org.example:client")
	assertDependency(t, client, "org.example:client", "[1.0]", "profiles.conditional.dependencies", RelationshipDirect, ScopeTest)
	if want := (map[string]string{"profile": "conditional", "conditional": "true", "maven_activation": activation, "maven_scope": "test", "classifier": "tests"}); !reflect.DeepEqual(client.Attributes, want) {
		t.Fatalf("client attributes = %+v, want %+v", client.Attributes, want)
	}
	bom := dependencyByName(t, result.Dependencies, "org.example:bom")
	assertDependency(t, bom, "org.example:bom", "[2.0]", "profiles.conditional.dependencyManagement", RelationshipInconclusive, ScopeBuild)
	if want := (map[string]string{"profile": "conditional", "conditional": "true", "maven_activation": activation, "maven_scope": "import", "type": "pom"}); !reflect.DeepEqual(bom.Attributes, want) {
		t.Fatalf("bom attributes = %+v, want %+v", bom.Attributes, want)
	}
	manual := dependencyByName(t, result.Dependencies, "org.example:manual")
	if want := (map[string]string{"profile": "manual", "conditional": "true"}); !reflect.DeepEqual(manual.Attributes, want) {
		t.Fatalf("manual attributes = %+v, want %+v", manual.Attributes, want)
	}
}

func TestMavenPOMProfileMissingIDUsesStablePosition(t *testing.T) {
	result, err := (mavenPOMParser{}).Analyze("pom.xml", []byte(`<project><profiles>
  <profile><id>empty</id></profile>
  <profile><dependencies><dependency><groupId>org.example</groupId><artifactId>client</artifactId><version>1.0</version></dependency></dependencies></profile>
</profiles></project>`))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionPartial}) || len(result.Dependencies) != 1 {
		t.Fatalf("result = %+v", result)
	}
	dependency := result.Dependencies[0]
	if dependency.SourceGroup != "profiles[2].dependencies" || dependency.Attributes["profile"] != "[2]" || dependency.Attributes["conditional"] != "true" {
		t.Fatalf("dependency = %+v", dependency)
	}
	if len(result.Diagnostics) != 1 || !strings.Contains(result.Diagnostics[0].Message, "profiles[2]") || !strings.Contains(result.Diagnostics[0].Message, "id") {
		t.Fatalf("diagnostics = %+v", result.Diagnostics)
	}
}

func TestMavenPOMProfileInvalidDependenciesAreReported(t *testing.T) {
	result, err := (mavenPOMParser{}).Analyze("pom.xml", []byte(`<project><profiles><profile><id>broken</id>
  <dependencies>
    <dependency><groupId>org.example</groupId><artifactId>client</artifactId><version>${missing}</version></dependency>
    <dependency><groupId>org.example</groupId></dependency>
  </dependencies>
  <dependencyManagement><dependencies><dependency><groupId>${missing.group}</groupId><artifactId>bom</artifactId><version>2.0</version></dependency></dependencies></dependencyManagement>
</profile></profiles></project>`))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionPartial}) || len(result.Dependencies) != 2 || len(result.Diagnostics) != 3 {
		t.Fatalf("result = %+v", result)
	}
	client := dependencyByName(t, result.Dependencies, "org.example:client")
	if client.Raw != "org.example:client:${missing}" || client.VersionConstraint != "${missing}" {
		t.Fatalf("client = %+v", client)
	}
	bom := dependencyByName(t, result.Dependencies, "${missing.group}:bom")
	if bom.Raw != "${missing.group}:bom:2.0" || bom.Attributes["profile"] != "broken" {
		t.Fatalf("bom = %+v", bom)
	}
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Code != incompleteExtractionCode || !strings.Contains(diagnostic.Message, "profiles.broken.") {
			t.Fatalf("diagnostic = %+v", diagnostic)
		}
	}
}

func TestMavenPOMProfileMalformedXMLFails(t *testing.T) {
	_, err := (mavenPOMParser{}).Analyze("pom.xml", []byte(`<project><profiles><profile><id>broken</id><dependencies></profile></profiles></project>`))
	if err == nil {
		t.Fatal("Analyze accepted malformed profile XML")
	}
}
