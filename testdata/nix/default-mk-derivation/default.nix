{ pkgs ? import <nixpkgs> {} }:
pkgs.stdenv.mkDerivation {
  pname = "acme-tool";
  version = "1.0.0";
  src = ./src;
  nativeBuildInputs = [ pkgs.cmake pkgs.pkg-config ];
  buildInputs = [ pkgs.libseccomp ];
  propagatedBuildInputs = [ pkgs.python3Packages.requests ];
  strictDeps = true;
}
