let
  source = builtins.fetchGit {
    url = "https://github.com/NixOS/nix.git";
    ref = "refs/tags/2.24.0";
    rev = "841fcbd04755c7a2865c51c1e2d3b045976b7452";
  };
in
import source {}
