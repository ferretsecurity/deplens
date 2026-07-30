{ pkgs ? import <nixpkgs> {} }:
pkgs.callPackage (
  { mkShell, hello, rustc, pkg-config, openssl }:
  mkShell {
    nativeBuildInputs = [ rustc pkg-config ];
    buildInputs = [ hello openssl ];
  }
) {}
