{ pkgs ? import <nixpkgs> {} }:
pkgs.stdenv.mkDerivation {
  pname = "fetched-source";
  version = "1.0.0";
  src = pkgs.fetchFromGitHub {
    owner = "acme";
    repo = "fetched-source";
    rev = "v1.0.0";
    hash = "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=";
  };
  nativeBuildInputs = [ pkgs.gnumake ];
}
