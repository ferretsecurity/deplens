package analyze

import (
	"net/url"
	"strings"
)

type pep508Dependency struct {
	name              string
	versionConstraint string
	originKind        OriginKind
	attributes        map[string]string
}

// parsePEP508Dep extracts the name, version constraint, extras, and marker
// from a PEP 508 dependency specifier. Raw input remains available separately
// on DependencyReference.Raw.
func parsePEP508Dep(spec string) pep508Dependency {
	spec = stripPyRequirementsComment(spec)
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
			if attributes := normalizePEP508Extras(rest[1:close]); attributes != "" {
				dep.attributes = map[string]string{"extras": attributes}
			}
			rest = strings.TrimSpace(rest[close+1:])
		}
	}

	if strings.HasPrefix(rest, "@") {
		target, marker := splitPEP508DirectMarker(strings.TrimSpace(rest[1:]))
		source := parsePythonDependencySource(target)
		if source.originKind == "" {
			return pep508Dependency{}
		}
		dep.originKind = source.originKind
		if dep.attributes == nil {
			dep.attributes = source.attributes
		} else {
			for key, value := range source.attributes {
				dep.attributes[key] = value
			}
		}
		rest = marker
	} else {
		dep.versionConstraint, rest = splitPEP508Marker(rest)
	}
	if rest != "" {
		if dep.attributes == nil {
			dep.attributes = make(map[string]string)
		}
		dep.attributes["marker"] = rest
	}
	return dep
}

func stripPyRequirementsComment(value string) string {
	var quote byte
	for i := 0; i < len(value); i++ {
		if quote != 0 && value[i] == '\\' && i+1 < len(value) {
			i++
			continue
		}
		switch value[i] {
		case '\'', '"':
			if quote == 0 {
				quote = value[i]
			} else if quote == value[i] {
				quote = 0
			}
		case '#':
			if quote == 0 && (i == 0 || value[i-1] == ' ' || value[i-1] == '\t') {
				return strings.TrimSpace(value[:i])
			}
		}
	}
	return strings.TrimSpace(value)
}

func splitPEP508DirectMarker(value string) (string, string) {
	for i := 1; i < len(value); i++ {
		if value[i] == ';' && (value[i-1] == ' ' || value[i-1] == '\t') {
			return strings.TrimSpace(value[:i]), strings.TrimSpace(value[i+1:])
		}
	}
	return value, ""
}

func parsePythonDependencySource(value string) pep508Dependency {
	if value == "" || strings.ContainsAny(value, " \t\r\n") {
		return pep508Dependency{}
	}
	localPath := !strings.Contains(value, "://") && !strings.ContainsAny(value, "<>=!~") && (strings.HasPrefix(value, ".") || strings.HasPrefix(value, "/") || strings.ContainsAny(value, `/\`) || strings.HasSuffix(value, ".whl") || strings.HasSuffix(value, ".tar.gz") || strings.HasSuffix(value, ".zip"))
	if strings.HasPrefix(value, "file:") || localPath {
		return pep508Dependency{originKind: OriginPath, attributes: map[string]string{"source_path": value}}
	}
	parsedURL, err := url.Parse(value)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return pep508Dependency{}
	}
	dep := pep508Dependency{originKind: OriginURL, attributes: map[string]string{"source_url": value}}
	if strings.HasPrefix(parsedURL.Scheme, "git+") || parsedURL.Scheme == "git" {
		dep.originKind = OriginGit
		if ref := strings.LastIndexByte(parsedURL.Path, '@'); ref >= 0 {
			dep.attributes["source_ref"] = parsedURL.Path[ref+1:]
			parsedURL.Path = parsedURL.Path[:ref]
			parsedURL.RawPath = ""
		}
	}
	fragment := make([]string, 0)
	for _, part := range strings.Split(parsedURL.EscapedFragment(), "&") {
		if egg, found := strings.CutPrefix(part, "egg="); found {
			if name, err := url.QueryUnescape(egg); err == nil {
				parsed := parsePEP508Dep(name)
				dep.name = parsed.name
				for key, value := range parsed.attributes {
					dep.attributes[key] = value
				}
			}
		} else {
			fragment = append(fragment, part)
		}
	}
	parsedURL.RawFragment = strings.Join(fragment, "&")
	parsedURL.Fragment, _ = url.PathUnescape(parsedURL.RawFragment)
	dep.attributes["source_url"] = parsedURL.String()
	return dep
}

func normalizePEP508Extras(value string) string {
	parts := strings.Split(value, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return strings.Join(parts, ",")
}

func splitPEP508Marker(value string) (versionConstraint, marker string) {
	var quote byte
	for i := 0; i < len(value); i++ {
		if quote != 0 && value[i] == '\\' && i+1 < len(value) {
			i++
			continue
		}
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
