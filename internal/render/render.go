package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/japroc/deplens/internal/analyze"
)

func Human(result analyze.ScanResult) string {
	if len(result.Manifests) == 0 {
		return fmt.Sprintf("Root: %s\nNo manifests found.\n", result.Root)
	}

	grouped := make(map[analyze.ManifestType][]string, len(result.Manifests))
	for _, manifest := range result.Manifests {
		grouped[manifest.Type] = append(grouped[manifest.Type], manifest.Path)
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Root: %s\n", result.Root))
	for _, manifestType := range analyze.SupportedManifestTypes() {
		paths := grouped[manifestType]
		if len(paths) == 0 {
			continue
		}
		b.WriteString(fmt.Sprintf("\n%s\n", manifestType))
		for _, path := range paths {
			b.WriteString(fmt.Sprintf("- %s\n", path))
		}
	}
	return b.String()
}

func JSON(result analyze.ScanResult) ([]byte, error) {
	var b bytes.Buffer
	encoder := json.NewEncoder(&b)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
