{ pkgs ? import <nixpkgs> {}, buildPackages ? pkgs.buildPackages, pkgsCross ? pkgs.pkgsCross.gnu64 }:
pkgs.stdenv.mkDerivation {
  pname = "cross-aware";
  version = "1.0";
  src = ./src;
  depsBuildBuild = [ buildPackages.stdenv.cc ];
  nativeBuildInputs = [ pkgs.pkg-config ];
  depsBuildTarget = [ pkgsCross.stdenv.cc ];
  depsHostHost = [ pkgs.stdenv.cc ];
  buildInputs = [ pkgs.zlib ];
  depsTargetTarget = [ pkgsCross.zlib ];
  propagatedNativeBuildInputs = [ pkgs.makeWrapper ];
  propagatedBuildInputs = [ pkgs.python3Packages.setuptools ];
  depsTargetTargetPropagated = [ pkgsCross.libffi ];
}
