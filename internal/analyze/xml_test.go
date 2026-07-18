package analyze

import "testing"

func TestXMLMatcherMatchesConfiguredPath(t *testing.T) {
	parser, err := newXMLMatcher(xmlMatcherConfig{
		ExistsAny: []string{"project.dependencies.dependency"},
	})
	if err != nil {
		t.Fatalf("newXMLMatcher failed: %v", err)
	}

	result, err := parser.Analyze("pom.xml", []byte(`
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
	if !result.Recognized {
		t.Fatalf("expected parser to match")
	}
	if result.Analysis.Presence != PresencePresent {
		t.Fatalf("expected presence=present, got %+v", result.Analysis)
	}
	if result.Dependencies != nil {
		t.Fatalf("expected no extracted dependencies, got %+v", result.Dependencies)
	}
}

func TestXMLMatcherReturnsFalseWhenPathMissing(t *testing.T) {
	parser, err := newXMLMatcher(xmlMatcherConfig{
		ExistsAny: []string{"project.dependencies.dependency"},
	})
	if err != nil {
		t.Fatalf("newXMLMatcher failed: %v", err)
	}

	result, err := parser.Analyze("pom.xml", []byte(`
<project>
  <modelVersion>4.0.0</modelVersion>
</project>
`))
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}
	if !result.Recognized {
		t.Fatalf("expected parser to match")
	}
	if result.Analysis.Presence != PresenceAbsent {
		t.Fatalf("expected presence=absent, got %+v", result.Analysis)
	}
	if result.Dependencies != nil {
		t.Fatalf("expected no extracted dependencies, got %+v", result.Dependencies)
	}
}

func TestXMLMatcherIgnoresNamespaces(t *testing.T) {
	parser, err := newXMLMatcher(xmlMatcherConfig{
		ExistsAny: []string{"project.dependencies.dependency"},
	})
	if err != nil {
		t.Fatalf("newXMLMatcher failed: %v", err)
	}

	result, err := parser.Analyze("pom.xml", []byte(`
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
	if !result.Recognized {
		t.Fatalf("expected parser to match")
	}
	if result.Analysis.Presence != PresencePresent {
		t.Fatalf("expected presence=present, got %+v", result.Analysis)
	}
}

func TestXMLMatcherRejectsMalformedXML(t *testing.T) {
	parser, err := newXMLMatcher(xmlMatcherConfig{
		ExistsAny: []string{"project.dependencies.dependency"},
	})
	if err != nil {
		t.Fatalf("newXMLMatcher failed: %v", err)
	}

	_, err = parser.Analyze("pom.xml", []byte(`<project><dependencies>`))
	if err == nil {
		t.Fatalf("expected parse error")
	}
}
