{ pkgs ? import <nixpkgs> {} }:
pkgs.mkShell {
  packages = [ pkgs.nodejs pkgs.pnpm ];
  inputsFrom = [ pkgs.hello pkgs.zlib ];
  shellHook = ''
    export ACME_DEVELOPMENT=1
  '';
}
