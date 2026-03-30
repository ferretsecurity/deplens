package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ferretsecurity/deplens/internal/analyze"
)

func Human(result analyze.ScanResult, supportedTypes []analyze.ManifestType) string {
	if len(result.Manifests) == 0 {
		return fmt.Sprintf("Root: %s\nNo manifests found.\n", result.Root)
	}

	grouped := make(map[analyze.ManifestType][]analyze.ManifestMatch, len(result.Manifests))
	for _, manifest := range result.Manifests {
		grouped[manifest.Type] = append(grouped[manifest.Type], manifest)
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Root: %s\n", result.Root))
	for _, manifestType := range supportedTypes {
		manifests := grouped[manifestType]
		if len(manifests) == 0 {
			continue
		}
		b.WriteString(fmt.Sprintf("\n%s\n", manifestType))
		for _, manifest := range manifests {
			b.WriteString(fmt.Sprintf("- %s\n", manifest.Path))
			for _, dependency := range manifest.Dependencies {
				b.WriteString(fmt.Sprintf("  - %s\n", dependency))
			}
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
