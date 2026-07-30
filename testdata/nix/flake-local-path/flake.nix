{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-24.11";
    local-tools.url = "path:./tools";
    remote = { url = "git+ssh://git@github.com/acme/remote.git?ref=main"; shallow = true; };
  };
  outputs = { self, nixpkgs, local-tools, remote }: { };
}
