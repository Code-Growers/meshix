{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/master";

    treefmt-nix.url = "github:numtide/treefmt-nix";
    nix2container = {
      url = "github:nlewo/nix2container";
      inputs.nixpkgs.follows = "nixpkgs";
    };

    make-shell.url = "github:nicknovitski/make-shell";
    globset = {
      url = "github:pdtpartners/globset";
      inputs.nixpkgs-lib.follows = "nixpkgs";
    };
  };

  outputs = inputs@{ self, nixpkgs, flake-parts, treefmt-nix, systems, globset, ... }:
    flake-parts.lib.mkFlake { inherit inputs; } {
      imports = [
        inputs.make-shell.flakeModules.default
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
          serverDocker = pkgs.callPackage ./nix/docker.nix {
            meshix-server = serverPkg;
          };
        in
        rec {
          formatter = treefmtEval.config.build.wrapper;
          checks = {
            # formatting = treefmtEval.config.build.check self';
          } // packages;
          packages = {
            server = serverPkg;
            client = clientPkg;
            default = serverPkg;
            server_docker = serverDocker;
          };

          make-shells.default = {
            env.LD_LIBRARY_PATH = "./ui/build/linux/x64/debug/plugins/flutter_pty/shared/";

            packages = [
              pkgs.buf
              pkgs.go
              pkgs.gopls
              pkgs.golint
              pkgs.goose
              pkgs.sqlc
              pkgs.flutter327
            ];
          };
        };
    };
}
