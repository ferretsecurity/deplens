# Security scan scope

Exclude the root `testdata/` directory from security analysis. It contains parser
fixtures, not dependencies installed to build or run deplens. Keep the root Go
module and Node tooling manifests in scope.

## Socket

The root `socket.yml` excludes `/testdata/` through `projectIgnorePaths`. Socket
uses gitignore-style patterns. Once this file reaches the scanned branch, check
the next repository report to verify fixture manifests are absent. Old reports
may still describe earlier commits.

Source: [Socket configuration](https://docs.socket.dev/docs/socket-yml).

## Snyk Open Source

Snyk does not support `.snyk` folder exclusions for managed dependency scans.
The hosted GitHub integration needs an import exclusion configured in Snyk:

1. In the `deplens` Snyk organization, open the GitHub repository import screen.
2. Add `testdata` to the folder exclusions at the bottom of the import window.
3. Import `ferretsecurity/deplens` with that exclusion. Retain the real root Go
   and Node dependency projects.
4. Deactivate existing fixture projects whose manifest paths start with
   `testdata/`. An import exclusion does not remove already-imported projects.
5. Check the active project list and subsequent scans to confirm that fixture
   projects are no longer scanned. Keep `testdata` excluded on future imports.

These hosted changes have not been applied from this workspace.

For a recursive CLI scan, explicitly pass the exclusion:

```sh
snyk test --all-projects --exclude=testdata
snyk monitor --all-projects --exclude=testdata
```

Sources: [Snyk exclusion FAQ](https://docs.snyk.io/manage-risk/prioritize-issues-for-fixing/ignore-issues/exclude-files-and-ignore-issues-faqs),
[import exclusions](https://docs.snyk.io/scan-with-snyk/import-project-repository/exclude-directories-and-files-from-project-import),
[CLI test options](https://docs.snyk.io/developer-tools/snyk-cli/commands/test).

## Snyk Code

The root `.snyk` policy excludes `testdata/**` from Snyk Code analysis. This is
separate from the Open Source import exclusion above. Check a Code rescan after
the policy reaches the scanned branch.

Source: [Snyk file exclusions](https://docs.snyk.io/developer-tools/snyk-cli/commands/ignore).
