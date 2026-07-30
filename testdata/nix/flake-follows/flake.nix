{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    home-manager = {
      url = "github:nix-community/home-manager";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    formatter = {
      url = "github:numtide/treefmt-nix";
      inputs.nixpkgs.follows = "home-manager/nixpkgs";
    };
  };
  outputs = { self, ... }: { };
}
