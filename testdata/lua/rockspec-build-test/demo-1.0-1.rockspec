package = "demo-build"
rockspec_format = "3.0"
version = "1.0-1"
source = { url = "git://example.test/demo-build.git", tag = "v1.0.0" }
dependencies = { "lua >= 5.1" }
build_dependencies = { "luacheck >= 1.1", "luarocks >= 3.9" }
test_dependencies = { "busted >= 2.2", "luacov" }
