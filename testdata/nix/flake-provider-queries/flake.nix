{
  inputs = {
    nixpkgs.url = "gitlab:NixOS/nixpkgs/nixos-24.11";
    source.url = "https://example.test/source.tar.gz?dir=nix&narHash=sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=";
    local.url = "./tools";
    local-path.url = "path:./tools";
    pinned.url = "git+https://github.com/acme/project.git?ref=main&rev=841fcbd04755c7a2865c51c1e2d3b045976b7452&dir=nix";
  };
  outputs = { self, ... }: { };
}
