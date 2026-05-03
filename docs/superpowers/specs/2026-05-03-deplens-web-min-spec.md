# Deplens Web Minimal Spec

## What is `deplens`

`deplens` is a scanner that walks a codebase and extracts dependency-related data across multiple ecosystems. It produces structured output that describes manifests and dependencies found in a repository.

## Purpose of `deplens-web`

`deplens-web` is the user-facing web application for the `deplens` platform. Its purpose is to let users navigate uploaded scan snapshots, browse repositories and projects, review dependency visibility over time, and work with lightweight metadata such as labels and annotations.

The web application is the presentation layer above `deplens-platform`. It does not ingest scans directly and does not perform scanning itself.

## Goals

- Provide a simple UI for navigating `deplens` scan results
- Let users browse data by tenant, project, repository, and snapshot
- Make historical scan results easier to inspect than raw JSON output
- Support labels and annotations as lightweight collaboration tools
- Work well for simple self-hosted deployments first
- Stay compatible with a later hosted multi-tenant SaaS deployment

## Non-goals

- Heavy analytics or reporting in the first version
- Complex workflow automation in the first version
- A separate frontend architecture for SaaS versus on-prem
- Rich cross-product dashboards beyond dependency visibility in the first version

## High-Level Tech Stack

- Frontend: React
- Language: TypeScript
- API integration: REST clients generated from OpenAPI when useful

The web application should remain a conventional single-page application that consumes the backend API.

## Deployment Model

### Initial deployment: on-prem

The first supported deployment model is self-hosted:

- the web app is deployed alongside `deplens-platform`
- it connects to the local platform API
- it should work well in a Docker Compose-based installation

For the first version, the self-hosted experience should feel simple and low-ops.

### Later deployment: SaaS

The same web application should later be usable in a hosted multi-tenant SaaS environment:

- same UI codebase
- same backend API model
- tenant-aware authentication and navigation

The move from self-hosted to SaaS should not require a separate frontend product.

## Tenancy Model

The UI must be tenant-aware from day 1.

### On-prem behavior

In self-hosted deployments, the system creates a default tenant on first run. If only one tenant exists, the UI should feel effectively single-tenant and avoid exposing unnecessary tenant management complexity.

### SaaS behavior

In SaaS mode, the UI should support users belonging to one or more tenants and should scope navigation and data views to the active tenant.

## Product Boundaries

`deplens-web` is responsible for:

- presenting projects, repositories, and scan snapshots
- showing historical scan data and comparisons
- exposing labels and annotations in the UI
- providing setup, login, and navigation flows

`deplens-web` is not responsible for:

- scanning repositories
- owning ingest logic
- storing scan artifacts directly
