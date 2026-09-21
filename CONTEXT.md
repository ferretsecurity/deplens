# Dependency visibility

Deplens describes dependency sources in a source tree and gaps in how another tool analyzes them.

## Language

**Dependency source**:
A file that can identify, declare, constrain, resolve, configure, use, or inventory dependencies.
_Avoid_: Dependency file, manifest

**Target SCA tool**:
The software composition analysis tool whose dependency coverage is being assessed.

**Source coverage gap**:
A dependency source that deplens recognizes but the target SCA tool cannot analyze.
_Avoid_: Unsupported dependency

**Reference coverage gap**:
A dependency reference that deplens finds but an actual scan by the target SCA tool omits.
_Avoid_: Unsupported dependency source

**Generated dependency source**:
A dependency source produced by deplens from declarations or other dependency references in an original source. It can be consumed by dependency tooling, including a target SCA tool; generation does not imply that dependency versions have been resolved.

**Independent dependency group**:
A set of dependency declarations within a source that belongs to its own environment. Different groups may have incompatible requirements, such as jobs that use different versions of the same package.
