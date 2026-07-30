{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-24.11";
    source = {
      url = "git+https://github.com/acme/source.git?ref=main&rev=841fcbd04755c7a2865c51c1e2d3b045976b7452";
      flake = false;
      submodules = true;
    };
  };
  outputs = { self, nixpkgs, source }: { };
}
