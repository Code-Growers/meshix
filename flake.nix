{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/master";
    devenv.url = "github:cachix/devenv";

    treefmt-nix.url = "github:numtide/treefmt-nix";
    nix2container = {
      url = "github:nlewo/nix2container";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    mk-shell-bin.url = "github:rrbutani/nix-mk-shell-bin";

    globset = {
      url = "github:pdtpartners/globset";
      inputs.nixpkgs-lib.follows = "nixpkgs";
    };
  };

  outputs = inputs@{ self, nixpkgs, flake-parts, treefmt-nix, systems, globset, ... }:
    flake-parts.lib.mkFlake { inherit inputs; } {
      imports = [
        inputs.devenv.flakeModule
      ];
      systems = [
        "x86_64-linux"
        "aarch64-darwin"
      ];

      perSystem = { config, self', inputs', pkgs, system, ... }:
        let
          treefmtEval = treefmt-nix.lib.evalModule pkgs ./nix/formatter.nix;
          serverPkg = pkgs.callPackage ./nix/server.nix {
            globset = globset;
          };
          clientPkg = pkgs.callPackage ./nix/client.nix {
            globset = globset;
          };
        in
        {
          formatter = treefmtEval.config.build.wrapper;
          checks = {
            # formatting = treefmtEval.config.build.check self';
          };
          packages = {
            server = serverPkg;
            client = clientPkg;
            default = serverPkg;
          };

          devenv.shells.default = {
            packages = [
              pkgs.buf
              pkgs.go
              pkgs.gopls
              pkgs.golint
              pkgs.goose
              pkgs.sqlc
            ];

          };
        };
    };
}
