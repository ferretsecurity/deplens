{
  inputs.self.submodules = true;
  inputs.self.lfs = true;
  inputs.nixpkgs.url = "github:NixOS/nixpkgs";
  outputs = { self, nixpkgs }: { };
}
