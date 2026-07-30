package = "demo-platform"
rockspec_format = "3.0"
version = "1.0-1"
source = { url = "https://example.test/demo-platform.tar.gz" }
dependencies = { "lua" }
platforms = { windows = { dependencies = { "luafilesystem >= 1.8" }, build_dependencies = { "win-build-tool >= 1.0" } }, linux = { dependencies = { "posix >= 36" }, external_dependencies = { READLINE = { library = "readline" } } } }
