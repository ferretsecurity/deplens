package analyze

import (
	"slices"
	"strings"
	"testing"
)

func TestPackageLockParserExtractsV1RootDependencies(t *testing.T) {
	parser, err := newPackageLockParser(packageLockMatcherConfig{})
	if err != nil {
		t.Fatalf("newPackageLockParser failed: %v", err)
	}

	result, err := parser.Match("package-lock.json", []byte(`
{
  "name": "demo",
  "lockfileVersion": 1,
  "dependencies": {
    "left-pad": {
      "version": "1.3.0"
    },
    "lodash": {
      "version": "4.17.21"
    }
  }
}
`))
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}
	if !result.Matched {
		t.Fatalf("expected match")
	}
	if want := []string{"left-pad@1.3.0", "lodash@4.17.21"}; !slices.Equal(dependencyNames(result.Dependencies), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", result.Dependencies, want)
	}
	if result.HasDependencies == nil || !*result.HasDependencies {
		t.Fatalf("expected has_dependencies=true, got %+v", result.HasDependencies)
	}
}

func TestPackageLockParserExtractsV3RootDependenciesAndOptionalDependenciesWithDedupe(t *testing.T) {
	parser, err := newPackageLockParser(packageLockMatcherConfig{})
	if err != nil {
		t.Fatalf("newPackageLockParser failed: %v", err)
	}

	result, err := parser.Match("package-lock.json", []byte(`
{
  "name": "demo",
  "lockfileVersion": 3,
  "packages": {
    "": {
      "name": "demo",
      "version": "1.0.0",
      "dependencies": {
        "left-pad": "^1.3.0"
      },
      "optionalDependencies": {
        "left-pad": "^1.3.0",
        "fsevents": "^2.3.3"
      }
    },
    "node_modules/left-pad": {
      "version": "1.3.0"
    },
    "node_modules/fsevents": {
      "version": "2.3.3"
    }
  }
}
`))
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}
	if !result.Matched {
		t.Fatalf("expected match")
	}
	if want := []string{"left-pad@1.3.0", "fsevents@2.3.3"}; !slices.Equal(dependencyNames(result.Dependencies), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", result.Dependencies, want)
	}
	if result.HasDependencies == nil || !*result.HasDependencies {
		t.Fatalf("expected has_dependencies=true, got %+v", result.HasDependencies)
	}
}

func TestPackageLockParserReturnsConclusiveEmptyWhenSupportedRootMapsAreMissing(t *testing.T) {
	parser, err := newPackageLockParser(packageLockMatcherConfig{})
	if err != nil {
		t.Fatalf("newPackageLockParser failed: %v", err)
	}

	result, err := parser.Match("package-lock.json", []byte(`
{
  "name": "demo",
  "lockfileVersion": 3,
  "packages": {
    "": {
      "name": "demo",
      "version": "1.0.0"
    }
  }
}
`))
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}
	if !result.Matched {
		t.Fatalf("expected match")
	}
	if result.Dependencies != nil {
		t.Fatalf("expected no dependencies, got %+v", result.Dependencies)
	}
	if result.HasDependencies == nil || *result.HasDependencies {
		t.Fatalf("expected has_dependencies=false, got %+v", result.HasDependencies)
	}
}

func TestPackageLockParserRejectsMalformedJSON(t *testing.T) {
	parser, err := newPackageLockParser(packageLockMatcherConfig{})
	if err != nil {
		t.Fatalf("newPackageLockParser failed: %v", err)
	}

	_, err = parser.Match("package-lock.json", []byte(`{"lockfileVersion": 3,`))
	if err == nil {
		t.Fatalf("expected parse error")
	}
	if got := err.Error(); !strings.Contains(got, "parse json file \"package-lock.json\"") {
		t.Fatalf("unexpected error: %v", err)
	}
}
