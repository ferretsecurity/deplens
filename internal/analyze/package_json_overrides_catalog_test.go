package analyze

import (
	"strings"
	"testing"
)

func TestPackageJSONBunCatalogOverridesPreserveDeclarations(t *testing.T) {
	for _, declaration := range []string{
		`"packageManager":"bun@1.3.0"`,
		`"workspaces":[],"catalog":{"react":"^19"}`,
		`"workspaces":[],"catalogs":{"frontend":{"react":"^19"}}`,
		`"workspaces":{"catalog":{"react":"^19"}}`,
		`"workspaces":{"catalogs":{"frontend":{"react":"^19"}}}`,
	} {
		t.Run(declaration, func(t *testing.T) {
			result, err := (packageJSONParser{}).Analyze("package.json", []byte(`{`+declaration+`,"overrides":{"parent@^1":{"react":"catalog:frontend"}}}`))
			if err != nil {
				t.Fatal(err)
			}
			if result.Analysis.Extraction != ExtractionComplete || len(result.Dependencies) != 1 || len(result.Diagnostics) != 0 {
				t.Fatalf("source = %+v", result)
			}
			dependency := result.Dependencies[0]
			if dependency.Raw != "react@catalog:frontend" || dependency.Name != "react" || dependency.SourceGroup != "overrides" ||
				dependency.Relationship != RelationshipInconclusive || dependency.OriginKind != "" || dependency.VersionConstraint != "" ||
				dependency.Attributes["override_path"] != `["parent@^1","react"]` {
				t.Fatalf("override = %+v", dependency)
			}
		})
	}
}

func TestPackageJSONNonBunCatalogOverridesStayUnsupported(t *testing.T) {
	for _, declaration := range []string{
		`"packageManager":"npm@11"`,
		`"packageManager":"pnpm@10"`,
		`"packageManager":"yarn@4"`,
		`"workspaces":["packages/*"]`,
	} {
		t.Run(declaration, func(t *testing.T) {
			result, err := (packageJSONParser{}).Analyze("package.json", []byte(`{`+declaration+`,"overrides":{"react":"catalog:"}}`))
			if err != nil {
				t.Fatal(err)
			}
			if result.Analysis.Extraction != ExtractionUnsupported || len(result.Dependencies) != 0 || len(result.Diagnostics) != 1 ||
				!strings.Contains(result.Diagnostics[0].Message, "unsupported replacement specifier") {
				t.Fatalf("source = %+v", result)
			}
		})
	}
}
