{
  inputs = {
    import-cargo = { type = "github"; owner = "edolstra"; repo = "import-cargo"; rev = "841fcbd04755c7a2865c51c1e2d3b045976b7452"; };
    nixpkgs = { type = "indirect"; id = "nixpkgs"; };
  };
  outputs = { self, ... }: { };
}
