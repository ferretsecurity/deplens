version = "0.8.0"
author = "demo"

# Nimble also accepts a local package directory as a dependency source.
requires "file:///opt/demo/shared-library"
requires "async-library[chronos, tracing]"

feature "sqlite":
  requires "nimsqlite3 >= 0.1.0"

dev:
  requires "unittest2"
