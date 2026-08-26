package analyze

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestBunLockFixturesExtractResolvedDependencies(t *testing.T) {
	testCases := []struct {
		name       string
		fixtureDir string
		want       []DependencyReference
	}{
		{
			name:       "runtime and development dependencies",
			fixtureDir: "bun-lock-runtime-and-dev",
			want: []DependencyReference{
				{PackageType: "npm", Raw: "scheduler@0.27.0", Name: "scheduler", Version: "0.27.0"},
				{PackageType: "npm", Raw: "react@19.2.3", Name: "react", Version: "19.2.3", SourceGroup: "dependencies"},
				{PackageType: "npm", Raw: "react-dom@19.2.3", Name: "react-dom", Version: "19.2.3", SourceGroup: "dependencies"},
				{PackageType: "npm", Raw: "@types/node@22.14.0", Name: "@types/node", Version: "22.14.0", SourceGroup: "devDependencies"},
			},
		},
		{
			name:       "configuration version",
			fixtureDir: "bun-lock-config-version",
			want: []DependencyReference{
				{PackageType: "npm", Raw: "bson@7.2.0", Name: "bson", Version: "7.2.0"},
				{PackageType: "npm", Raw: "mongodb@7.0.0", Name: "mongodb", Version: "7.0.0"},
				{PackageType: "npm", Raw: "mongoose@9.2.1", Name: "mongoose", Version: "9.2.1", SourceGroup: "dependencies"},
			},
		},
		{
			name:       "development and peer dependencies",
			fixtureDir: "bun-lock-dev-and-peer",
			want: []DependencyReference{
				{PackageType: "npm", Raw: "bun-types@1.2.21", Name: "bun-types", Version: "1.2.21"},
				{PackageType: "npm", Raw: "@types/bun@1.2.21", Name: "@types/bun", Version: "1.2.21", SourceGroup: "devDependencies"},
				{PackageType: "npm", Raw: "bun-plugin-dts@0.3.0", Name: "bun-plugin-dts", Version: "0.3.0", SourceGroup: "devDependencies"},
				{PackageType: "npm", Raw: "typescript@5.9.2", Name: "typescript", Version: "5.9.2", SourceGroup: "peerDependencies"},
			},
		},
	}

	ruleset := mustLoadDefaultRules(t)
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Scan(filepath.Join("..", "..", "testdata", "javascript", tc.fixtureDir), nil, ruleset)
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			if len(result.Sources) != 1 {
				t.Fatalf("sources = %+v, want one", result.Sources)
			}
			source := result.Sources[0]
			if source.Detector != "js-bun-lock" || source.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) {
				t.Fatalf("source = %+v", source)
			}
			if !reflect.DeepEqual(source.Dependencies, tc.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, tc.want)
			}
		})
	}
}

func TestBunLockParserReturnsConclusiveEmptyForEmptyPackages(t *testing.T) {
	parser, err := newBunLockParser(bunLockMatcherConfig{})
	if err != nil {
		t.Fatalf("newBunLockParser: %v", err)
	}

	result, err := parser.Analyze("bun.lock", []byte(`{"lockfileVersion": 1, "packages": {}}`))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if !result.Recognized || result.Analysis != (SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}) || len(result.Dependencies) != 0 {
		t.Fatalf("result = %+v", result)
	}
}
