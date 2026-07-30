package = "demo-scm"
rockspec_format = "3.0"
version = "scm-1"
source = { url = "git+https://github.com/example/demo-scm.git", branch = "main" }
dependencies = { "lua >= 5.2", "inspect ~> 3.1" }
build = { type = "builtin", modules = { ["demo"] = "src/demo.lua" } }
