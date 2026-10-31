# Repository scanning design

The design decisions below are agreed. The complete design awaits a final shared-understanding review. Implementation has not been approved.

## Agreed scope

Collect dependency inventories across selected hosted repositories and save results and metadata on the filesystem. How consumers compare or process those inventories is outside this tool's scope.

The workflow includes shallow cloning, scanning with deplens, and cleaning up the checkout. Configuration supports including and excluding projects and groups, including subgroups.

The first version scans each repository's default branch.

Do not fetch submodules or Git LFS content in the first version. Scan the repository's own checked-out files and record that scope in metadata. Submodule repositories can be selected separately.

Build a separate companion tool that invokes the deplens CLI.

Implement the companion as a separate Go project and repository, invoking an installed deplens binary. Keep its release cycle separate from deplens. Finish the design in this repository before creating the companion project.

Each invocation processes the selected repositories and exits. Scheduling is outside the tool's scope. Users choose and configure cron, systemd, a CI pipeline, or another scheduler themselves.

The first version supports both GitHub and GitLab.

Support self-hosted instances through a configurable server URL. One invocation can use multiple named connections, including multiple connections to the same server with different accounts. Each connection has a provider, server URL, credential reference, and independent inclusion/exclusion settings.

Use one YAML configuration file for connections, selections, scan settings, and execution limits. Credentials remain in environment variables.

## Repository selection

Selections use exact repository and group paths, without wildcard matching in the first version. Including a GitLab group includes projects throughout its subgroup tree. A group exclusion covers its entire subtree. GitHub selection uses organizations and individual repositories.

Exclusions always win, including over an explicit repository inclusion. Configuration order does not affect selection.

Archived repositories and forks are excluded by default. Separate options allow users to include each category.

Each connection must explicitly select repositories, groups, or organizations, or opt into all accessible repositories. If neither is provided, report a configuration error. Selecting all accessible repositories still respects exclusions and the archived-repository and fork settings.

Keep selection and reuse state separate per named connection, even when connections select the same repository. Within one connection, process each repository once regardless of how many selections match. Identify repositories by server and provider repository ID, retaining current paths as metadata so history survives renames.

## Authentication

Each connection names an environment variable containing its token. Use the token for API discovery and HTTPS cloning, with only the access needed to discover and read the selected repositories.

Do not put tokens in clone URLs, command-line arguments, logs, or stored results. SSH cloning and automatic token renewal are outside the first version's scope.

## Reusing results

Use one deplens scan configuration, including rules and flags, for all repositories in an invocation. Per-repository overrides are outside the first version's scope.

Reuse a previous complete result when the repository identity, target commit, deplens version, and effective scan configuration match. Compare custom rule contents, not just their file paths.

Resolve the current remote commit before cloning so unchanged repositories with reusable results need neither a clone nor a scan.

Record the time the repository was checked separately from the time it was scanned. A skipped scan refers to its previous result.

Failed and partial attempts do not qualify for reuse and are retried on subsequent runs. Unsupported extraction and reported coverage gaps do not prevent reuse. Actual analysis failures and partial extraction do prevent reuse; a successful process exit alone is insufficient.

## Result storage

Keep each actual scan's result and metadata, with a reference to the latest complete result. Reusing a result does not duplicate the inventory. Automatic retention cleanup is outside the first version's scope.

Store deplens output and scan metadata as separate JSON files. Each batch has a JSON summary referencing its saved results, including reused results. No database is required.

Only one invocation may use a results directory at a time. Lock that directory and reject a second invocation with a clear error. Workers within the owning invocation share access.

Keep saved history when a repository disappears or is no longer selected, but exclude it from the current batch's inventory. Record each batch's selected repositories and outcomes so consumers can distinguish current results from historical ones.

## Execution

The initial deployment is a virtual machine scanning around 10,000 repositories, ranging from very small projects to large monorepos with more than ten years of history. Disk space is sufficient for results, but not necessarily for all source checkouts.

Batch completion is best effort, with no initial completion deadline. Measure actual duration before setting a performance target.

Concurrency is configurable and defaults to three workers. Each worker uses one temporary checkout, saves the result, and removes the checkout before processing another repository. Shallow cloning limits history, but does not bound the size of the current source tree.

Use a configurable per-repository timeout, defaulting to 60 minutes for cloning and scanning together. If an attempt exceeds its timeout or cannot continue because of disk space, stop the attempt, record the failure, clean up its checkout, and continue processing other repositories when resources permit.

Stop starting new clones when free disk space is below a configured threshold, defaulting to 10 GiB. This threshold does not guarantee that active clones cannot fill the disk. If space remains below the threshold after active workers finish and clean up, end the batch with an error and record unprocessed repositories rather than waiting indefinitely. Tune these starting limits after measuring actual batches.

Continue processing other repositories after individual failures and save each failure. Finish with a nonzero exit code if any repository failed or discovery was incomplete. Save a batch summary with counts of scanned, reused, and failed repositories.

After a crash or restart, the next invocation removes abandoned checkouts owned by this tool, reuses completed results, and retries unfinished work. No separate resume command is required.

Allow up to three attempts for transient API or clone failures, with increasing delays. Respect server-requested delays within a bounded wait. Do not retry invalid credentials or permission errors within the batch. Clone retries count toward the per-repository timeout.

Record empty repositories with an `empty` outcome without cloning or scanning. This is not a failure. Inaccessible repositories are failures, not empty repositories.

## Design review

No interview questions remain open. Confirm shared understanding of the complete design before implementation. Concrete file names, configuration keys, and other routine implementation details should follow these decisions without adding scope.
