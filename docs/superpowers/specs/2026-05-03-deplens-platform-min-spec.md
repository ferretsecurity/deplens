# Deplens Platform Minimal Spec

## What is `deplens`

`deplens` is a scanner that walks a codebase and extracts dependency-related data across multiple ecosystems. It produces structured output that describes manifests and dependencies found in a repository.

## Purpose of `deplens-platform`

`deplens-platform` is the central backend service for `deplens` results. Its purpose is to accept uploaded scan snapshots with repository and project metadata, store them as a system of record, and expose them through an API so users can navigate scan history over time.

The platform is not a replacement for `deplens`. `deplens` produces the data. `deplens-platform` stores, organizes, and serves it.

## Goals

- Provide a central place to upload `deplens` scan results
- Store repository and project metadata alongside each scan
- Treat each upload as an immutable scan snapshot
- Make scan history queryable by tenant, project, repository, and time
- Support light metadata overlays such as labels and annotations
- Start with simple on-prem installation
- Preserve a clean path to a later hosted SaaS offering

## Non-goals

- Full ASPM scope
- Policy engines, alerting, or automated remediation in the first version
- Microservices in the first version
- Per-tenant database isolation in the first version
- Complex distributed ingestion infrastructure in the first version

## High-Level Tech Stack

- Backend: Go
- API: REST, with OpenAPI used for API description and client generation
- Database: PostgreSQL
- Artifact storage: pluggable blob storage interface
- Blob backends:
  - local filesystem
  - S3-compatible object storage

The preferred shape is a modular monolith rather than multiple services.

## Deployment Model

### Initial deployment: on-prem

The first supported deployment model is self-hosted and operationally simple:

- one application service
- one PostgreSQL database
- local filesystem storage for uploaded artifacts by default

This should be easy to run in Docker Compose. Operators may optionally use S3-compatible local object storage later, but it should not be required for the first install.

### Later deployment: SaaS

The same application should later run as a hosted multi-tenant SaaS product:

- same application codebase
- managed PostgreSQL, for example Amazon RDS for PostgreSQL
- S3-compatible object storage, for example Amazon S3

The deployment model should change through configuration, not through a separate architecture.

## Tenancy Model

Tenancy must be built in from day 1.

The platform uses logical multi-tenancy with shared application tables scoped by `tenant_id`. This allows the same codebase to support both self-hosted and SaaS deployments.

### On-prem behavior

On first run, a self-hosted installation creates a default tenant automatically. This default tenant is used for the initial local deployment so the product feels single-tenant in practice, even though the system is tenant-aware underneath.

### SaaS behavior

In SaaS mode, multiple tenants share the same application deployment and database, with all application data scoped by tenant.

## Product Boundaries

`deplens-platform` is responsible for:

- authenticating users or uploaders
- accepting uploaded scan snapshots
- associating scans with tenants, projects, and repositories
- storing raw uploaded artifacts
- storing normalized scan metadata for querying and navigation

`deplens-platform` is not responsible for scanning repositories itself.
