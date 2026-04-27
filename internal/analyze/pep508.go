package analyze

import "strings"

// parsePEP508Dep splits a PEP 508 dependency specifier into the bare package
// name and the remainder (extras + version constraint + environment marker).
func parsePEP508Dep(spec string) (name, rest string) {
	if idx := strings.Index(spec, " #"); idx >= 0 {
		spec = strings.TrimSpace(spec[:idx])
	}
	if spec == "" {
		return "", ""
	}
	end := 0
	for end < len(spec) {
		c := spec[end]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') ||
			(c >= '0' && c <= '9') || c == '.' || c == '-' || c == '_' {
			end++
		} else {
			break
		}
	}
	name = spec[:end]
	rest = strings.TrimSpace(spec[end:])
	return name, rest
}
