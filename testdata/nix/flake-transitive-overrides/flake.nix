{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-24.11";
    nixops.url = "github:NixOS/nixops";
    nixops.inputs.nixpkgs = { type = "github"; owner = "acme"; repo = "nixpkgs"; };
    home-manager.url = "github:nix-community/home-manager";
    home-manager.inputs.nixpkgs.follows = "nixpkgs";
  };
  outputs = { self, ... }: { };
}
