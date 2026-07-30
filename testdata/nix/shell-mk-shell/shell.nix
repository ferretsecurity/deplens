{ pkgs ? import <nixpkgs> {} }:
pkgs.mkShell {
  nativeBuildInputs = [ pkgs.rustc pkgs.pkg-config ];
  buildInputs = [ pkgs.openssl pkgs.zlib ];
  nativeCheckInputs = [ pkgs.cargo-nextest ];
  checkInputs = [ pkgs.libxml2 ];
}
