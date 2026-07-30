version = "0.5.0"
author = "demo"

requires "nim >= 2.0.0", "fusion >= 1.2.0"

when defined(windows):
  requires "winim >= 3.9.0"
elif defined(macosx):
  requires "cocoanigui >= 0.1.0"
else:
  requires "posixutils >= 0.3.0"
