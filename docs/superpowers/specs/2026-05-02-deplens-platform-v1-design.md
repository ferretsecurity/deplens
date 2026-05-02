# Deplens platform v1 design

## Summary

This design defines the first version of a central service for `deplens` scan results. `deplens` remains the scanner and producer of dependency inventory data. The new platform accepts immutable scan snapshots plus repository metadata, stores the raw uploaded artifact, normalizes selected fields into PostgreSQL for query and navigation, and exposes a web UI and REST API for engineering and security teams.

The first release is optimized for simple on-prem installation. The same codebase must also remain viable for a later hosted SaaS deployment with multiple tenants. To support that, multi-tenancy is built in from day 1, but self-hosted installs bootstrap a single default tenant automatically on first run.

## Goals

- Provide a central service where `deplens` results can be uploaded with repo and project metadata
- Treat every upload as an immutable scan snapshot
- Make scan history queryable and navigable by tenant, project, repository, and time
- Support light mutable metadata on top of immutable snapshots through labels and annotations
- Keep first-time self-hosted deployment simple enough for Docker Compose
- Preserve a clean path to later hosted multi-tenant SaaS operation

## Non-goals

- Full application security posture management
- Alerting, policy engines, or automated remediation in v1
- Deep dependency graph modeling across manifests
- Per-tenant database isolation in v1
- A microservice architecture in v1
- Real-time streaming ingest or message-bus-based processing

## Product boundaries

`deplens` is responsible for:

- scanning a codebase
- producing machine-readable JSON output
- optionally collecting local metadata before upload

The platform is responsible for:

- authenticating uploaders and users
- resolving uploaded data into a tenant, project, and repository hierarchy
- storing immutable scan snapshots
- storing the raw uploaded artifact
- extracting normalized, queryable dependency records
- presenting history, diffs, search, labels, and annotations in the UI and API

The platform does not replace `deplens`. It is the system of record and navigation layer above `deplens`.

## Recommended stack

### Backend

- Go
- modular monolith
- REST API with OpenAPI as the source of truth
- PostgreSQL as the system of record
- pluggable blob storage interface with filesystem and S3-compatible backends

Recommended Go libraries:

- router: `chi`
- postgres driver: `pgx`
- query generation: `sqlc`
- migrations: `golang-migrate`
- OpenAPI generation: `oapi-codegen` or equivalent

### Frontend

- React
- TypeScript
- Vite
- React Router
- TanStack Query

The frontend is a separate web app that talks to the backend through the REST API.

### Storage

- PostgreSQL stores normalized relational state and search dimensions
- blob storage stores raw uploaded scan artifacts
- supported blob backends:
  - local filesystem for simple on-prem installs
  - S3-compatible object storage for SaaS and advanced self-hosted installs

## Why a modular monolith

The primary workload is append-only ingest plus read-heavy browsing. That does not justify microservice overhead in v1. A modular monolith keeps the install small, reduces coordination complexity, and still supports later extraction of subsystems if one area proves operationally distinct.

The app should be one deployable binary with clear internal domain boundaries:

- `auth`
- `tenancy`
- `projects`
- `repositories`
- `ingest`
- `scans`
- `dependencies`
- `annotations`
- `storage`
- `admin`

## Tenancy model

### Decision

Use logical multi-tenancy with shared PostgreSQL tables keyed by `tenant_id`.

### Rationale

- simplest on-prem deployment
- same schema works for self-hosted and SaaS
- avoids operational cost of per-tenant databases
- keeps migrations and reporting simple

### Self-hosted behavior

On first startup, if no tenants exist:

1. create a `default` tenant
2. create the first admin user in that tenant
3. mark the installation as initialized

If the instance only has one tenant, the UI may hide tenant switching even though the underlying model remains tenant-aware.

### Enforcement

- almost every business table includes `tenant_id`
- every authenticated request resolves an active tenant
- all repository, project, snapshot, dependency, label, and annotation queries scope by `tenant_id`
- unique constraints should generally be per-tenant, not global

Row-level security is optional future defense-in-depth. It is not required for v1 if tenant scoping is explicit and consistently tested in application code.

## Deployment model

### First self-hosted target

The supported starter deployment is Docker Compose with:

- `app`
- `postgres`

Optional:

- `minio` when the operator wants S3-compatible object storage locally

Default self-hosted storage profile:

- PostgreSQL in a local container
- blob storage on a local filesystem volume mounted into `app`

### SaaS target

The same application should run later with:

- app container on the chosen compute platform
- Amazon RDS for PostgreSQL
- S3 for blob storage

The codepath should not branch by deployment type. Only configuration changes.

## Core data model

The domain hierarchy is:

- tenant
- project
- repository
- scan snapshot

Mutable metadata layers on top:

- labels
- annotations

### Entities

#### `tenants`

Represents an isolated customer or organizational boundary.

Suggested fields:

- `id`
- `slug`
- `name`
- `created_at`
- `updated_at`

#### `users`

Represents an authenticated actor.

Suggested fields:

- `id`
- `email`
- `display_name`
- `password_hash` or external identity reference
- `created_at`
- `updated_at`

#### `memberships`

Maps users to tenants and roles.

Suggested fields:

- `id`
- `tenant_id`
- `user_id`
- `role`
- `created_at`

#### `projects`

Logical grouping inside a tenant.

Suggested fields:

- `id`
- `tenant_id`
- `key`
- `name`
- `description`
- `created_at`
- `updated_at`

Constraints:

- unique (`tenant_id`, `key`)

#### `repositories`

Represents a source repository or codebase location within a project.

Suggested fields:

- `id`
- `tenant_id`
- `project_id`
- `key`
- `name`
- `origin_url`
- `default_branch`
- `created_at`
- `updated_at`

Constraints:

- unique (`tenant_id`, `key`)

#### `scan_snapshots`

Represents one immutable upload event.

Suggested fields:

- `id`
- `tenant_id`
- `project_id`
- `repository_id`
- `branch`
- `commit_sha`
- `tag`
- `ci_run_id`
- `uploaded_by_user_id` nullable
- `uploader_name` nullable
- `deplens_version`
- `schema_version`
- `scanned_at`
- `uploaded_at`
- `manifest_count`
- `dependency_count`
- `warning_count`
- `artifact_id`
- `status`

Constraints:

- append-only after creation
- no updates except internal lifecycle fields if ingest becomes asynchronous later

#### `scan_artifacts`

Represents the stored raw uploaded artifact.

Suggested fields:

- `id`
- `tenant_id`
- `storage_backend`
- `storage_key`
- `content_type`
- `size_bytes`
- `sha256`
- `created_at`

#### `scan_manifests`

Stores normalized manifest-level records extracted from the upload.

Suggested fields:

- `id`
- `tenant_id`
- `scan_snapshot_id`
- `manifest_type`
- `path`
- `has_dependencies`
- `warning_count`

#### `scan_dependencies`

Stores normalized dependency records extracted from each manifest.

Suggested fields:

- `id`
- `tenant_id`
- `scan_snapshot_id`
- `scan_manifest_id`
- `name`
- `raw`
- `version`
- `constraint`
- `section`
- `source`
- `extras_json`

This table mirrors the `deplens` dependency structure closely so that ingest stays straightforward and query semantics remain predictable.

#### `labels`

Represents reusable labels scoped to a tenant.

Suggested fields:

- `id`
- `tenant_id`
- `name`
- `color`
- `created_at`

#### `label_bindings`

Attaches labels to mutable targets.

Suggested fields:

- `id`
- `tenant_id`
- `label_id`
- `target_type`
- `target_id`
- `created_at`

Supported initial targets:

- project
- repository
- scan snapshot

#### `annotations`

Stores lightweight mutable user notes.

Suggested fields:

- `id`
- `tenant_id`
- `target_type`
- `target_id`
- `body`
- `created_by_user_id`
- `created_at`
- `updated_at`

Supported initial targets:

- repository
- scan snapshot
- scan manifest
- scan dependency

## Indexing and query strategy

Baseline indexes:

- `projects (tenant_id, key)`
- `repositories (tenant_id, project_id)`
- `repositories (tenant_id, key)`
- `scan_snapshots (tenant_id, repository_id, scanned_at desc)`
- `scan_snapshots (tenant_id, project_id, scanned_at desc)`
- `scan_manifests (tenant_id, scan_snapshot_id)`
- `scan_dependencies (tenant_id, scan_snapshot_id)`
- `scan_dependencies (tenant_id, name)`

Optional later indexes:

- trigram or full-text support for dependency name search
- partial indexes for frequently filtered status fields

PostgreSQL should remain the only query engine in v1. Do not add OpenSearch or Elasticsearch before query behavior proves insufficient.

## Ingest model

### Upload contract

The platform accepts:

- repo and project identity
- source metadata like branch and commit
- scanner metadata like `deplens` version
- optional labels
- raw `deplens` JSON payload

Illustrative request:

```json
{
  "tenant_slug": "default",
  "project_key": "payments",
  "repository_key": "github.com/acme/payments-api",
  "repository_name": "payments-api",
  "branch": "main",
  "commit_sha": "abc123",
  "ci_run_id": "build-9821",
  "scanned_at": "2026-05-02T10:00:00Z",
  "deplens_version": "0.4.0",
  "schema_version": "1",
  "labels": [
    "prod",
    "tier1"
  ],
  "result": {
    "root": "/workspace",
    "manifests": []
  }
}
```

### Ingest steps

1. authenticate caller
2. resolve active tenant
3. validate payload shape and supported schema version
4. upsert or resolve project and repository
5. write the raw artifact to blob storage
6. create `scan_artifacts`
7. create immutable `scan_snapshots`
8. normalize manifests and dependencies into relational tables
9. attach requested labels
10. compute summary counters
11. return snapshot identity and summary

### Sync vs async

Start with synchronous ingest in the request path. The expected v1 scale does not justify queue infrastructure.

If ingest later becomes too slow, split the lifecycle into:

- artifact accepted
- snapshot pending normalization
- snapshot ready

That can still be implemented with PostgreSQL-backed jobs inside the same binary.

## Snapshot immutability

Immutability is a product rule, not only a UI rule.

Allowed after creation:

- adding labels
- adding annotations

Not allowed after creation:

- changing raw uploaded data
- changing normalized manifest or dependency records
- changing repo/project association
- changing scanner metadata

If a scan was uploaded with bad metadata, the correction path should be “upload a new snapshot” unless the mistake is purely administrative and affects only a mutable overlay.

## API design

### Principles

- REST first
- OpenAPI spec stored in the backend repo
- generated clients for the web app and future CLI integrations
- tenant-aware endpoints with explicit authentication context

### Initial resources

- auth
- users
- tenants
- projects
- repositories
- scan snapshots
- manifests
- dependencies
- labels
- annotations

### Initial endpoint sketch

Authentication and bootstrap:

- `POST /api/v1/auth/login`
- `POST /api/v1/auth/logout`
- `GET /api/v1/me`
- `POST /api/v1/setup/initialize`

Tenant and membership:

- `GET /api/v1/tenants`
- `POST /api/v1/tenants`
- `GET /api/v1/tenants/{tenantId}`

Projects and repositories:

- `GET /api/v1/projects`
- `POST /api/v1/projects`
- `GET /api/v1/projects/{projectId}`
- `GET /api/v1/repositories`
- `POST /api/v1/repositories`
- `GET /api/v1/repositories/{repositoryId}`

Ingest and scan history:

- `POST /api/v1/scan-snapshots`
- `GET /api/v1/scan-snapshots`
- `GET /api/v1/scan-snapshots/{snapshotId}`
- `GET /api/v1/repositories/{repositoryId}/scan-snapshots`
- `GET /api/v1/scan-snapshots/{snapshotId}/manifests`
- `GET /api/v1/scan-snapshots/{snapshotId}/dependencies`
- `GET /api/v1/scan-snapshots/{snapshotId}/diff?against={snapshotId}`

Labels and annotations:

- `GET /api/v1/labels`
- `POST /api/v1/labels`
- `POST /api/v1/label-bindings`
- `GET /api/v1/annotations`
- `POST /api/v1/annotations`

### Diff behavior

The first diff view should answer:

- which manifests were added or removed
- which dependency names were added or removed

Do not start with a graph diff or transitive impact model. A set-based diff between snapshots is enough for v1.

## Frontend scope

The initial web UI should include:

- first-run setup screen
- login flow
- project and repository navigation
- repository overview
- scan history list
- snapshot detail page
- snapshot diff view
- dependency search and filtering
- labels and annotations UI
- basic admin pages for tenant and user context

If the installation only has one tenant, the UI should feel single-tenant by default while preserving internal tenant context.

## Authentication

The first release should optimize for simple self-hosted deployment.

Recommended v1:

- local user accounts with email and password
- server-side session cookies or signed access tokens

Recommended near-term extension:

- OIDC for enterprise and SaaS environments

Do not make external identity a hard requirement for the first self-hosted release.

## Blob storage interface

Define a narrow backend interface, for example:

```go
type BlobStore interface {
    Put(ctx context.Context, key string, contentType string, body io.Reader) (BlobRef, error)
    Get(ctx context.Context, key string) (io.ReadCloser, BlobMeta, error)
    Delete(ctx context.Context, key string) error
}
```

Supported implementations:

- filesystem
- S3-compatible

Blob keys should include tenant and snapshot identity so that migrations and operator troubleshooting remain straightforward.

Example key shape:

```text
tenants/<tenant-id>/snapshots/<snapshot-id>/deplens-result.json
```

## Configuration model

The application should support environment-variable configuration first.

Required areas:

- server bind address
- public base URL
- database DSN
- blob backend kind
- filesystem blob root or S3 credentials and bucket
- bootstrap mode flags
- auth/session secrets

Avoid a complex config language in v1. Keep runtime setup legible in Docker Compose and cloud secret managers.

## Recommended repository organization

### GitHub organization

Create or use a dedicated GitHub organization for product-level repos. Keep the scanner and platform separate.

Recommended initial repositories:

- `deplens`
- `deplens-platform`
- `deplens-web`

Optional later repository:

- `deplens-api`

Only split `deplens-api` out if multiple clients begin to depend on a shared release cadence. Until then, keep the OpenAPI spec inside `deplens-platform`.

### Why separate repos

`deplens` has a different lifecycle and audience from the hosted platform. The scanner should remain independently usable and releasable. The backend and frontend can evolve faster without coupling every change to the CLI repository.

### Backend repo structure

Suggested layout for `deplens-platform`:

```text
cmd/
  server/
  migrate/
internal/
  admin/
  annotations/
  auth/
  config/
  db/
  dependencies/
  http/
  ingest/
  projects/
  repositories/
  scans/
  storage/
  tenancy/
migrations/
openapi/
deploy/
  docker-compose/
```

### Frontend repo structure

Suggested layout for `deplens-web`:

```text
src/
  api/
  app/
  components/
  features/
    admin/
    annotations/
    projects/
    repositories/
    scans/
  routes/
```

### GitHub project organization

Use one GitHub Project for the platform and organize work into streams:

- Platform foundation
- Auth and tenancy
- Ingest pipeline
- Scan browsing and diffing
- Deployment and packaging

Suggested early milestones:

- `v0.1` self-hosted bootstrap
- `v0.2` upload and snapshot browse
- `v0.3` dependency search and diff
- `v0.4` labels and annotations
- `v0.5` SaaS-readiness hardening

## First release scope

### Must-have

- first-run setup with default tenant bootstrap
- user login
- create and browse projects and repositories
- upload immutable `deplens` snapshots
- persist raw artifact plus normalized manifests and dependencies
- browse repository scan history
- inspect snapshot details
- diff one snapshot against another
- add labels and annotations
- run in Docker Compose with local PostgreSQL

### Should-have

- filesystem and S3 blob backends
- generated TypeScript API client from OpenAPI
- repository-level filtering by branch and time

### Explicitly deferred

- SSO and enterprise role mapping
- policy evaluation
- alerting workflows
- webhooks
- background queue infrastructure
- advanced full-text or graph indexing
- per-tenant database or cluster isolation

## Risks and mitigations

### Risk: ingestion schema drift

`deplens` JSON will evolve over time.

Mitigation:

- version the upload schema
- record the `deplens` version and schema version on every snapshot
- keep raw artifacts so normalization can be replayed later if needed

### Risk: tenant isolation bugs

Shared-table tenancy is simple but unforgiving if filters are inconsistent.

Mitigation:

- require `tenant_id` in nearly every table
- centralize tenant resolution and query helpers
- add tests that prove cross-tenant isolation on every resource type

### Risk: artifact growth

Raw JSON artifacts may grow faster than relational state.

Mitigation:

- keep artifacts out of PostgreSQL row storage
- retain only references and summaries in the database
- support S3-compatible storage from the start

### Risk: on-prem complexity creep

SaaS preparation can easily bloat the first install.

Mitigation:

- keep the first deployment to one app plus Postgres
- make advanced storage and identity options additive, not mandatory

## Open questions intentionally left out of v1

These are valid future topics but should not block the initial design:

- OIDC provider support shape
- role granularity beyond basic tenant admin and member
- whether labels need hierarchy or only flat tags
- artifact retention policies
- whether snapshot diffs should later include version-change classification

## Success criteria

- a self-hosted operator can start the platform with Docker Compose and create the first admin plus default tenant
- a `deplens` client can upload a scan snapshot with repo and project metadata through one API call
- the platform stores the raw uploaded artifact outside PostgreSQL and stores normalized searchable records in PostgreSQL
- users can browse scan history by repository and inspect a single snapshot
- users can compare two snapshots and see added and removed manifests and dependencies
- labels and annotations work as mutable overlays without changing the immutable snapshot record
- the same application can switch from filesystem blobs plus local PostgreSQL to S3 plus RDS through configuration only
