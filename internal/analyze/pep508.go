package analyze

import "strings"

type pep508Dependency struct {
	name       string
	constraint string
	extras     map[string]string
}

// parsePEP508Dep extracts the name, version constraint, extras, and marker
// from a PEP 508 dependency specifier. Raw input remains available separately
// on Dependency.Raw.
func parsePEP508Dep(spec string) pep508Dependency {
	if idx := strings.Index(spec, " #"); idx >= 0 {
		spec = strings.TrimSpace(spec[:idx])
	}
	if spec == "" {
		return pep508Dependency{}
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
	dep := pep508Dependency{name: spec[:end]}
	rest := strings.TrimSpace(spec[end:])
	if strings.HasPrefix(rest, "[") {
		if close := strings.IndexByte(rest, ']'); close >= 0 {
			if extras := normalizePEP508Extras(rest[1:close]); extras != "" {
				dep.extras = map[string]string{"extras": extras}
			}
			rest = strings.TrimSpace(rest[close+1:])
		}
	}

	dep.constraint, rest = splitPEP508Marker(rest)
	if rest != "" {
		if dep.extras == nil {
			dep.extras = make(map[string]string)
		}
		dep.extras["marker"] = rest
	}
	return dep
}

func normalizePEP508Extras(value string) string {
	parts := strings.Split(value, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return strings.Join(parts, ",")
}

func splitPEP508Marker(value string) (constraint, marker string) {
	var quote byte
	for i := 0; i < len(value); i++ {
		switch value[i] {
		case '\'', '"':
			if quote == 0 {
				quote = value[i]
			} else if quote == value[i] {
				quote = 0
			}
		case ';':
			if quote == 0 {
				return strings.TrimSpace(value[:i]), strings.TrimSpace(value[i+1:])
			}
		}
	}
	return strings.TrimSpace(value), ""
}
