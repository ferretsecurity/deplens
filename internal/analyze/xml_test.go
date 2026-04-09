package analyze

import "testing"

func TestXMLMatcherMatchesConfiguredPath(t *testing.T) {
	parser, err := newXMLMatcher(xmlMatcherConfig{
		ExistsAny: []string{"project.dependencies.dependency"},
	})
	if err != nil {
		t.Fatalf("newXMLMatcher failed: %v", err)
	}

	deps, hasDependencies, ok, err := parser.Match("pom.xml", []byte(`
<project>
  <dependencies>
    <dependency>
      <groupId>org.slf4j</groupId>
      <artifactId>slf4j-api</artifactId>
      <version>2.0.17</version>
    </dependency>
  </dependencies>
</project>
`))
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected parser to match")
	}
	if hasDependencies == nil || !*hasDependencies {
		t.Fatalf("expected has_dependencies=true, got %+v", hasDependencies)
	}
	if deps != nil {
		t.Fatalf("expected no extracted dependencies, got %+v", deps)
	}
}

func TestXMLMatcherReturnsFalseWhenPathMissing(t *testing.T) {
	parser, err := newXMLMatcher(xmlMatcherConfig{
		ExistsAny: []string{"project.dependencies.dependency"},
	})
	if err != nil {
		t.Fatalf("newXMLMatcher failed: %v", err)
	}

	deps, hasDependencies, ok, err := parser.Match("pom.xml", []byte(`
<project>
  <modelVersion>4.0.0</modelVersion>
</project>
`))
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected parser to match")
	}
	if hasDependencies == nil || *hasDependencies {
		t.Fatalf("expected has_dependencies=false, got %+v", hasDependencies)
	}
	if deps != nil {
		t.Fatalf("expected no extracted dependencies, got %+v", deps)
	}
}

func TestXMLMatcherIgnoresNamespaces(t *testing.T) {
	parser, err := newXMLMatcher(xmlMatcherConfig{
		ExistsAny: []string{"project.dependencies.dependency"},
	})
	if err != nil {
		t.Fatalf("newXMLMatcher failed: %v", err)
	}

	_, hasDependencies, ok, err := parser.Match("pom.xml", []byte(`
<project xmlns="http://maven.apache.org/POM/4.0.0">
  <dependencies>
    <dependency>
      <groupId>org.slf4j</groupId>
      <artifactId>slf4j-api</artifactId>
      <version>2.0.17</version>
    </dependency>
  </dependencies>
</project>
`))
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected parser to match")
	}
	if hasDependencies == nil || !*hasDependencies {
		t.Fatalf("expected has_dependencies=true, got %+v", hasDependencies)
	}
}

func TestXMLMatcherRejectsMalformedXML(t *testing.T) {
	parser, err := newXMLMatcher(xmlMatcherConfig{
		ExistsAny: []string{"project.dependencies.dependency"},
	})
	if err != nil {
		t.Fatalf("newXMLMatcher failed: %v", err)
	}

	_, _, _, err = parser.Match("pom.xml", []byte(`<project><dependencies>`))
	if err == nil {
		t.Fatalf("expected parse error")
	}
}
