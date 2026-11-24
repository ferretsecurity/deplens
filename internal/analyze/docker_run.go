package analyze

import (
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/moby/buildkit/frontend/dockerfile/instructions"
	"github.com/moby/buildkit/frontend/dockerfile/parser"
	"mvdan.cc/sh/v3/syntax"
)

// A nonliteral word keeps its position so it cannot turn an argument into a
// command or an option value into a package.
type dockerRunWord struct {
	value   string
	literal bool
}

// Docker appends the RUN text as one argument to the configured SHELL.
func dockerShellAcceptsScript(node *parser.Node) bool {
	if !node.Attributes["json"] || node.Next == nil {
		return false
	}
	switch path.Base(node.Next.Value) {
	case "sh", "bash", "ash", "dash", "ksh":
	default:
		return false
	}
	for arg := node.Next.Next; arg != nil; arg = arg.Next {
		if arg.Value == "-o" || arg.Value == "+o" {
			arg = arg.Next
			if arg == nil {
				return false
			}
			continue
		}
		flags, ok := strings.CutPrefix(arg.Value, "-")
		if !ok || flags == "" || strings.Trim(flags, "abefhiklmnptuvxcBCEHPT") != "" || strings.Contains(flags, "n") {
			return false
		}
		if strings.Contains(flags, "c") {
			return arg.Next == nil
		}
	}
	return false
}

func dockerRunDependencies(cmd *instructions.RunCommand) ([]DependencyReference, []string) {
	if !cmd.PrependShell {
		words := make([]dockerRunWord, len(cmd.CmdLine))
		for i, word := range cmd.CmdLine {
			words[i] = dockerRunWord{word, true}
		}
		return dockerInstallDependencies(words), nil
	}
	script := strings.Join(cmd.CmdLine, " ")
	if len(cmd.Files) == 1 && parser.MustParseHeredoc(script) != nil {
		file := cmd.Files[0]
		script = file.Data
		if file.Chomp {
			script = parser.ChompHeredocContent(script)
		}
		// A shebang can select an arbitrary interpreter.
		if strings.HasPrefix(script, "#!") {
			return nil, nil
		}
	} else if len(cmd.Files) > 0 {
		// Empty heredocs preserve the command's redirections without parsing data.
		var command strings.Builder
		command.WriteString(script)
		for _, file := range cmd.Files {
			command.WriteByte('\n')
			command.WriteString(file.Name)
		}
		script = command.String()
	}
	tree, err := syntax.NewParser().Parse(strings.NewReader(script), "")
	if err != nil {
		return nil, []string{fmt.Sprintf("Dockerfile RUN shell could not be parsed: %v", err)}
	}
	var dependencies []DependencyReference
	functions := make(map[string]bool)
	syntax.Walk(tree, func(node syntax.Node) bool {
		switch node := node.(type) {
		case *syntax.FuncDecl:
			functions[node.Name.Value] = true
			return false
		case *syntax.CmdSubst, *syntax.ProcSubst, *syntax.Redirect:
			return false
		case *syntax.CallExpr:
			words := make([]dockerRunWord, len(node.Args))
			for i, word := range node.Args {
				words[i].value, words[i].literal = dockerLiteralParts(word.Parts, false)
			}
			if len(words) > 0 && !functions[words[0].value] {
				dependencies = append(dependencies, dockerInstallDependencies(words)...)
			}
			return false

		}
		return true
	})
	return dependencies, nil
}

func dockerLiteralParts(parts []syntax.WordPart, quoted bool) (string, bool) {
	var value strings.Builder
	for _, part := range parts {
		switch part := part.(type) {
		case *syntax.Lit:
			for i := 0; i < len(part.Value); i++ {
				ch := part.Value[i]
				if ch == '\\' && i+1 < len(part.Value) {
					next := part.Value[i+1]
					if !quoted || strings.ContainsRune("$`\"\\\n", rune(next)) {
						i++
						if next != '\n' {
							value.WriteByte(next)
						}
						continue
					}
				}
				if !quoted && (strings.ContainsRune("*?[{}", rune(ch)) || ch == '~' && i == 0 && value.Len() == 0) {
					return "", false
				}
				value.WriteByte(ch)
			}
		case *syntax.SglQuoted:
			if part.Dollar {
				return "", false
			}
			value.WriteString(part.Value)
		case *syntax.DblQuoted:
			if part.Dollar {
				return "", false
			}
			text, ok := dockerLiteralParts(part.Parts, true)
			if !ok {
				return "", false
			}
			value.WriteString(text)
		default:
			return "", false
		}
	}
	return value.String(), true
}

var dockerPythonExecutable = regexp.MustCompile(`^(?:python|pip)(?:[0-9]+(?:\.[0-9]+)*)?$`)
var dockerDebPackage = regexp.MustCompile(`^([a-z0-9][a-z0-9+.-]*(?::[a-z0-9][a-z0-9-]*)?)(=[A-Za-z0-9.+:~_-]+)?$`)
var dockerApkPackage = regexp.MustCompile(`^([A-Za-z0-9][A-Za-z0-9+_.-]*)((?:=|~=|>=|<=|>|<|~)[A-Za-z0-9.+:_-]+)?$`)

type dockerOptions struct {
	flags, valued []string
}

// consumed returns the number of argv words used by an option, or zero if
// its arity is unknown. Short flags can be clustered, as in apt-get -yqq.
func (spec dockerOptions) consumed(value string) int {
	if strings.HasPrefix(value, "--") {
		option, _, attached := strings.Cut(value, "=")
		switch {
		case slices.Contains(spec.valued, option):
			if !attached {
				return 2
			}
			return 1
		case !attached && slices.Contains(spec.flags, option):
			return 1
		}
		return 0
	}
	if len(value) < 2 {
		return 0
	}
	for i := 1; i < len(value); i++ {
		option := "-" + value[i:i+1]
		if slices.Contains(spec.valued, option) {
			if i+1 == len(value) {
				return 2
			}
			return 1
		}
		if !slices.Contains(spec.flags, option) {
			return 0
		}
	}
	return 1
}

var dockerInstallOptions = map[string]dockerOptions{
	"deb": {
		flags: strings.Fields(`-y --yes --assume-yes -q --quiet --no-install-recommends --no-install-suggests
			--allow-downgrades --allow-change-held-packages --allow-unauthenticated --reinstall
			--download-only -d --fix-broken -f --no-remove --only-upgrade --no-upgrade
			--ignore-missing --fix-missing --update`),
		valued: strings.Fields(`-o --option -c --config-file -t --target-release --snapshot`),
	},
	"apk": {
		flags: strings.Fields(`--no-cache --update-cache -U --upgrade -u --latest -l --quiet -q --verbose -v
			--allow-untrusted --no-progress --initdb --no-scripts --simulate -s --update`),
		valued: strings.Fields(`--virtual -t --repository -X --root -p --arch --keys-dir --repositories-file --cache-dir`),
	},
	"pypi": {
		flags: strings.Fields(`--upgrade -U --no-cache-dir --break-system-packages --no-index --no-deps --user --pre
			--force-reinstall --ignore-installed -I --quiet -q --verbose -v
			--disable-pip-version-check --no-build-isolation --prefer-binary --require-hashes
			--isolated --no-cache --no-compile --no-warn-script-location`),
		valued: strings.Fields(`-r --requirement -c --constraint -e --editable -f --find-links -i --index-url
			--extra-index-url --trusted-host --target -t --prefix --root --proxy --timeout --retries
			--cert --client-cert --cache-dir --log --global-option --install-option --config-settings
			-C --only-binary --no-binary --upgrade-strategy --root-user-action --progress-bar
			--platform --python-version --implementation --abi --report --python`),
	},
}

func dockerInstallDependencies(words []dockerRunWord) []DependencyReference {
	if len(words) == 0 || !words[0].literal {
		return nil
	}
	executable := words[0].value
	if strings.Contains(executable, "/") && !strings.HasPrefix(executable, "/") {
		return nil
	}
	executable = path.Base(executable)
	packageType, verb := "", "install"
	switch {
	case executable == "apt" || executable == "apt-get":
		packageType = "deb"
	case executable == "apk":
		packageType = "apk"
		verb = "add"
	case dockerPythonExecutable.MatchString(executable):
		packageType = "pypi"
		if strings.HasPrefix(executable, "python") {
			if len(words) < 4 || !words[1].literal || words[1].value != "-m" || !words[2].literal || words[2].value != "pip" {
				return nil
			}
			words = words[2:]
		}
	default:
		return nil
	}
	installed := false
	options := true
	var dependencies []DependencyReference
	for i := 1; i < len(words); i++ {
		word := words[i]
		if !word.literal {
			// An expansion can supply a flag that consumes the next operand.
			if options {
				return dependencies
			}
			continue
		}
		value := word.value
		if options && value == "--" {
			options = false
			continue
		}
		if options && strings.HasPrefix(value, "-") {
			n := dockerInstallOptions[packageType].consumed(value)
			if n == 0 || i+n > len(words) {
				return dependencies
			}
			i += n - 1
			continue
		}
		if !installed {
			if value != verb {
				return nil
			}
			installed = true
			continue
		}
		dep, ok := dockerRunPackage(packageType, value)
		if ok {
			dep.SourceGroup = "RUN"
			dep.Relationship = RelationshipDirect
			dep.Scope = ScopeBuild
			dependencies = append(dependencies, dep)
		}
	}
	return dependencies
}

func dockerRunPackage(packageType, raw string) (DependencyReference, bool) {
	if packageType == "pypi" {
		if strings.ContainsAny(raw, "$`\\") {
			return DependencyReference{}, false
		}
		// Archives and local project paths are not registry requirements.
		lower := strings.ToLower(raw)
		for _, suffix := range []string{".zip", ".whl", ".tar.bz2", ".tbz", ".tar.gz", ".tgz", ".tar", ".tar.xz", ".txz", ".tlz", ".tar.lz", ".tar.lzma"} {
			if strings.HasSuffix(lower, suffix) {
				return DependencyReference{}, false
			}
		}
		dep, err := parsePythonRequirement(raw)
		return dep, err == nil
	}
	pattern := dockerDebPackage
	if packageType == "apk" {
		pattern = dockerApkPackage
	}
	match := pattern.FindStringSubmatch(raw)
	if match == nil {
		return DependencyReference{}, false
	}
	return DependencyReference{PackageType: PackageType(packageType), Raw: raw, Name: match[1], VersionConstraint: match[2], OriginKind: OriginRegistry}, true
}
