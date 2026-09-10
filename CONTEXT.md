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
