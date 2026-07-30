package = "demo-external"
rockspec_format = "3.0"
version = "1.0-1"
source = { url = "https://example.test/demo-external.tar.gz" }
dependencies = { "lua >= 5.4" }
external_dependencies = { OPENSSL = { header = "openssl/ssl.h", library = "ssl" }, ZLIB = { library = "z" } }
