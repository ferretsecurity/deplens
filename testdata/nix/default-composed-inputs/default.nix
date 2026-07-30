{ pkgs ? import <nixpkgs> {}, lib ? pkgs.lib }:
with pkgs;
stdenv.mkDerivation {
  pname = "composed-inputs";
  version = "1.0.0";
  src = ./src;
  nativeBuildInputs = [ cmake pkg-config ] ++ lib.optionals stdenv.isDarwin [ darwin.apple_sdk.frameworks.Security ];
  buildInputs = [ openssl ] ++ lib.optional stdenv.isLinux systemd;
}
