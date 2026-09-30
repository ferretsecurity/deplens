package analyze

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// BenchmarkScanPNPMLock includes rule selection, extraction, normalization, and checks.
func BenchmarkScanPNPMLock(b *testing.B) {
	for _, count := range []int{1000, 10000, 30000} {
		b.Run(fmt.Sprintf("packages-%d", count), func(b *testing.B) {
			root := b.TempDir()
			var content strings.Builder
			content.WriteString("lockfileVersion: '9.0'\npackages:\n")
			for i := 0; i < count; i++ {
				fmt.Fprintf(&content, "  package-%05d@1.2.3:\n    resolution: {integrity: sha512-fixture}\n", i)
			}
			if err := os.WriteFile(filepath.Join(root, "pnpm-lock.yaml"), []byte(content.String()), 0600); err != nil {
				b.Fatal(err)
			}
			rules, err := LoadDefaultRules()
			if err != nil {
				b.Fatal(err)
			}
			verify := func(result ScanResult) {
				b.Helper()
				if len(result.Sources) != 1 {
					b.Fatalf("sources: %+v", result.Sources)
				}
				source := result.Sources[0]
				if source.Detector != "js-pnpm-lock" || source.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) || len(source.Diagnostics) != 0 || len(source.Dependencies) != count {
					b.Fatalf("unexpected pnpm source: %+v", source)
				}
				for i, dep := range source.Dependencies {
					name := fmt.Sprintf("package-%05d", i)
					if dep.Name != name || dep.Raw != name+"@1.2.3" || dep.Version != "1.2.3" || dep.PackageType != "npm" {
						b.Fatalf("dependency %d: %+v", i, dep)
					}
				}
			}
			result, err := Scan(root, nil, rules)
			if err != nil {
				b.Fatal(err)
			}
			verify(result)
			b.ReportAllocs()
			b.SetBytes(int64(content.Len()))
			b.ResetTimer()
			for b.Loop() {
				result, err = Scan(root, nil, rules)
				if err != nil {
					b.Fatal(err)
				}
			}
			b.StopTimer()
			verify(result)
		})
	}
}
