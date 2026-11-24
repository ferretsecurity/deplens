package analyze

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
)

func TestDockerfileRunPackages(t *testing.T) {
	tests := []struct {
		name, recipe string
		want         []string
	}{
		{"chains", `RUN apt-get update && apt-get install -y jq curl=8.0 && apk add --no-cache bash=5.2 && pip install 'requests==2.32'`, []string{"apk:bash=5.2", "deb:curl=8.0", "deb:jq", "pypi:requests==2.32"}},
		{"executables", `RUN /usr/bin/apt install -y jq; /sbin/apk add bash; /venv/bin/python3.12 -m pip install --no-index -f /dist pyghidra; /usr/bin/pip3.12 install torch`, []string{"apk:bash", "deb:jq", "pypi:pyghidra", "pypi:torch"}},
		{"exec", `RUN ["/usr/bin/pip3", "install", "requests>=2", "--index-url", "https://example.test/simple", "torch"]`, []string{"pypi:requests>=2", "pypi:torch"}},
		{"ambiguous option", `RUN opt=-r; pip install numpy "$opt" requirements.txt; apk add gcc --mystery value`, []string{"apk:gcc", "pypi:numpy"}},
		{"shell arguments", "SHELL [\"/bin/sh\", \"-c\", \"echo\"]\nRUN pip install fake\nSHELL [\"bash\", \"-o\", \"pipefail\", \"-lc\"]\nRUN pip install real --no-warn-script-location", []string{"pypi:real"}},
		{"function shadow", `RUN pip() { echo "$@"; }; pip install fake; /usr/bin/pip install real`, []string{"pypi:real"}},
		{"positions", `RUN echo apt-get install fake; printf '%s' 'pip install fake'; apt-get update; pip download fake; python -c 'pip install fake'`, nil},
		{"options", `RUN apt-get -o A=B install --no-install-recommends -y jq; apk --root /tmp add --virtual .build gcc; pip --proxy http://proxy install -r requirements.txt -c constraints.txt --target /app --find-links /dist --upgrade torch`, []string{"apk:gcc", "deb:jq", "pypi:torch"}},
		{"unknown options", `RUN apt install --mystery possible-value fake; apk add --unknown fake; pip install --unknown fake`, nil},
		{"dynamic", `RUN apt install safe "${PKG}"; apk add safe $PACKAGES; pip install safe "requests==$VERSION"`, []string{"apk:safe", "deb:safe", "pypi:safe"}},
		{"paths", `RUN pip install . ../thing /tmp/pkg dist/pkg.whl pkg.TAR.BZ2 pkg.tgz pkg.tar.xz https://example.test/pkg git+https://example.test/pkg torch; apt install ./local.deb /tmp/file.deb; apk add /tmp/file.apk`, []string{"pypi:torch"}},
		{"quoted words", `RUN pip install 'requests>=2,<3' "httpx[http2]==0.27"; apt install 'libc6:amd64=2.0'`, []string{"deb:libc6:amd64=2.0", "pypi:httpx[http2]==0.27", "pypi:requests>=2,<3"}},
		{"custom shell", "SHELL [\"echo\"]\nRUN pip install fake\nRUN [\"pip\", \"install\", \"real\"]", []string{"pypi:real"}},
		{"duplicate references", "RUN apt install curl; apt-get install curl; apk add curl", []string{"apk:curl", "deb:curl"}},
		{"observed options", "RUN apt install --update -yqq jq; apk add --update curl; pip install --no-cache torch", []string{"apk:curl", "deb:jq", "pypi:torch"}},
		{"heredoc data", "RUN cat <<'DATA'\npip install fake\napt install fake\nDATA\nRUN pip install real\n", []string{"pypi:real"}},
		{"heredoc script", "RUN <<'SH'\napt install -y jq\npip install torch\nSH\n", []string{"deb:jq", "pypi:torch"}},
		{"continuation", "RUN --mount=type=cache,target=/cache apt-get install -y \\\n jq \\\n curl\n", []string{"deb:curl", "deb:jq"}},
		{"short option ambiguity", `RUN pip install -vunknown fake; apt install -yunknown fake; apk add -qunknown fake`, nil},
		{"attached option values", `RUN pip install -rrequirements.txt -f/dist torch; apt -oA=B install jq; apk add -t.build gcc`, []string{"apk:gcc", "deb:jq", "pypi:torch"}},
		{"heredoc substitution data", "RUN cat <<DATA\n$(apt install fake)\nDATA\nRUN pip install real\n", []string{"pypi:real"}},
		{"heredoc script expansion", "RUN <<SH\napt install jq $PACKAGES\npip install torch\nSH\n", []string{"deb:jq", "pypi:torch"}},
		{"redirect-only heredoc data", "RUN true; <<DATA\napt install fake\nDATA\n", nil},
		{"function definitions", `RUN install_deps() { apt install fake; }; pip install real`, []string{"pypi:real"}},
		{"glob and substitutions", "RUN pip install safe req* \"$(echo fake)\" `echo fake`; echo $(apt install nested)", []string{"pypi:safe"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analyzer, _ := newDockerfileParser(dockerfileMatcherConfig{})
			result, err := analyzer.Analyze("Dockerfile", []byte("FROM scratch\n"+tt.recipe+"\n"))
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, dep := range result.Dependencies {
				got = append(got, string(dep.PackageType)+":"+dep.Raw)
				if dep.SourceGroup != "RUN" || dep.Relationship != RelationshipDirect || dep.OriginKind != OriginRegistry {
					t.Errorf("metadata: %+v", dep)
				}
			}
			slices.Sort(got)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestScanDockerfileRunPackages(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "Dockerfile"), []byte("FROM --reset=all alpine:3.20\nRUN apk add --no-cache bash=5.2\nRUN pip install 'requests==2.32'\nCOPY --from=busybox:1 /bin/busybox /bin/busybox\n"), 0600); err != nil {
		t.Fatal(err)
	}
	result, err := Scan(root, nil, mustLoadDefaultRules(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("sources: %+v", result.Sources)
	}
	source := result.Sources[0]
	if source.Analysis.Extraction != ExtractionComplete || len(source.Dependencies) != 4 {
		t.Fatalf("source: %+v", source)
	}
	for _, dep := range source.Dependencies {
		if dep.Raw == "requests==2.32" && (dep.PackageType != "pypi" || dep.VersionConstraint != "==2.32" || dep.VERS != "vers:pypi/2.32") {
			t.Fatalf("requirement: %+v", dep)
		}
	}
}

func TestDockerfileRunPinMetadata(t *testing.T) {
	analyzer, _ := newDockerfileParser(dockerfileMatcherConfig{})
	result, err := analyzer.Analyze("Dockerfile", []byte("FROM scratch\nRUN apt install libc6:amd64=2.36-9; apk add 'bash>=5.2'; pip install 'requests[security]==2.32'\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Dependencies) != 3 {
		t.Fatalf("dependencies: %+v", result.Dependencies)
	}
	for _, dep := range result.Dependencies {
		if dep.SourceGroup != "RUN" || dep.Scope != ScopeBuild {
			t.Errorf("metadata: %+v", dep)
		}
		switch dep.PackageType {
		case "deb":
			if dep.Name != "libc6:amd64" || dep.VersionConstraint != "=2.36-9" || dep.Version != "" {
				t.Errorf("Debian pin: %+v", dep)
			}
		case "apk":
			if dep.Name != "bash" || dep.VersionConstraint != ">=5.2" {
				t.Errorf("APK constraint: %+v", dep)
			}
		case "pypi":
			if dep.Name != "requests" || dep.VersionConstraint != "==2.32" || dep.Attributes["extras"] != "security" {
				t.Errorf("Python requirement: %+v", dep)
			}
		default:
			t.Errorf("unexpected reference: %+v", dep)
		}
	}
}
