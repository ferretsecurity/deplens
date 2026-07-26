package analyze

import (
	"strings"
	"testing"
)

func TestDockerfileParserExtractsStagesArgsAndExternalCopyImages(t *testing.T) {
	parser, _ := newDockerfileParser(dockerfileMatcherConfig{})
	result, err := parser.Analyze("Dockerfile", []byte(`
ARG GO_IMAGE=golang:1.24-alpine
FROM ${GO_IMAGE} AS builder
RUN go build -o /out/app
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /out/app /app
COPY --from=ghcr.io/acme/assets:2.0 /assets /assets
`))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(result.Dependencies) != 3 {
		t.Fatalf("unexpected dependencies: %+v", result.Dependencies)
	}
	for _, dependency := range result.Dependencies {
		switch dependency.Name {
		case "golang":
			if dependency.Scope != ScopeBuild || dependency.Attributes["stage"] != "builder" {
				t.Fatalf("unexpected builder image: %+v", dependency)
			}
		case "gcr.io/distroless/static-debian12":
			if dependency.Scope != ScopeRuntime {
				t.Fatalf("unexpected final image: %+v", dependency)
			}
		case "ghcr.io/acme/assets":
			if dependency.SourceGroup != "COPY --from" || dependency.Scope != ScopeBuild {
				t.Fatalf("unexpected copy image: %+v", dependency)
			}
		}
	}
}

func TestDockerfileParserPreservesDigestAndPlatform(t *testing.T) {
	parser, _ := newDockerfileParser(dockerfileMatcherConfig{})
	result, err := parser.Analyze("Dockerfile", []byte(`FROM --platform=linux/amd64 ghcr.io/acme/app@sha256:abc123`))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	dependency := result.Dependencies[0]
	if dependency.Attributes["digest"] != "sha256:abc123" || dependency.Attributes["platform"] != "linux/amd64" {
		t.Fatalf("unexpected image attributes: %+v", dependency)
	}
}

func TestDockerfileParserDoesNotTreatPriorStageAsImage(t *testing.T) {
	parser, _ := newDockerfileParser(dockerfileMatcherConfig{})
	result, err := parser.Analyze("Dockerfile", []byte(`
FROM golang:1.24 AS builder
FROM builder AS packaged
COPY --from=builder /out/app /app
`))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(result.Dependencies) != 1 || result.Dependencies[0].Name != "golang" || result.Dependencies[0].Scope != ScopeBuild {
		t.Fatalf("unexpected dependencies: %+v", result.Dependencies)
	}
}

func TestDockerfileParserReportsUnresolvedImages(t *testing.T) {
	parser, _ := newDockerfileParser(dockerfileMatcherConfig{})
	result, err := parser.Analyze("Dockerfile", []byte(`
FROM alpine:3.20 AS helper
FROM ${FINAL_IMAGE}
`))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.Analysis.Extraction != ExtractionPartial || len(result.Dependencies) != 1 || result.Dependencies[0].Scope != ScopeBuild {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestDockerfileParserRejectsUnterminatedContinuation(t *testing.T) {
	parser, _ := newDockerfileParser(dockerfileMatcherConfig{})
	_, err := parser.Analyze("Dockerfile", []byte("FROM alpine:3.20 \\\n"))
	if err == nil || !strings.Contains(err.Error(), "unterminated") {
		t.Fatalf("expected continuation error, got %v", err)
	}
}

func TestDockerComposeParserExtractsImagesAndReportsInterpolation(t *testing.T) {
	parser, _ := newDockerComposeParser(dockerComposeMatcherConfig{})
	result, err := parser.Analyze("compose.yaml", []byte(`
services:
  api:
    image: ghcr.io/acme/api:2.4.0
  database:
    image: postgres:16.3
  worker:
    image: ${WORKER_IMAGE:-worker:latest}
`))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.Analysis.Extraction != ExtractionPartial || len(result.Dependencies) != 2 || len(result.Diagnostics) != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestDockerComposeParserRecognizesFilesWithoutImagesAsEmpty(t *testing.T) {
	parser, _ := newDockerComposeParser(dockerComposeMatcherConfig{})
	result, err := parser.Analyze("compose.yaml", []byte(`
services:
  api:
    build: .
`))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.Analysis != (SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}) {
		t.Fatalf("unexpected analysis: %+v", result.Analysis)
	}
}

func TestDockerComposeParserRejectsNonMappingServices(t *testing.T) {
	parser, _ := newDockerComposeParser(dockerComposeMatcherConfig{})
	_, err := parser.Analyze("compose.yaml", []byte("services: []\n"))
	if err == nil || !strings.Contains(err.Error(), "must be a mapping") {
		t.Fatalf("expected services error, got %v", err)
	}
}
