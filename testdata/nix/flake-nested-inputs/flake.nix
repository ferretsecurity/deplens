{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    rust-overlay.url = "github:oxalica/rust-overlay";
    crane = { url = "github:ipetkov/crane"; };
    utils.url = "github:numtide/flake-utils";
  };
}
