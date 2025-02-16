{ dockerTools
, stdenvNoCC
, meshix-server
, meshix-client
, buildEnv
, coreutils
, nix
, bash
}:
let
  configurations = stdenvNoCC.mkDerivation {
    name = "server-configuration";

    src = ./../server/configuration;

    installPhase = ''
      mkdir -p $out/configuration
      cp * $out/configuration
    '';
  };
  migrations = stdenvNoCC.mkDerivation {
    name = "server-migrations";

    src = ./../server/migrations;

    installPhase = ''
      mkdir -p $out/migrations
      cp * $out/migrations
    '';
  };

  image-with-certs = dockerTools.buildImage {
    name = "image-with-certs";
    tag = "latest";

    copyToRoot = buildEnv {
      name = "image-with-certs-root";
      paths = [
        coreutils
        dockerTools.caCertificates
      ];
    };

    config = { };
  };

in
{
  server =
    dockerTools.streamLayeredImage {
      name = "meshix-server";
      tag = "0.0.1-rc0";
      contents = [
        configurations
        migrations
      ];
      config = {
        Entrypoint = [ "${meshix-server}/bin/${meshix-server.meta.mainProgram}" ];
      };
    };
  client =
    dockerTools.buildImage {
      name = "meshix-client";
      tag = "0.0.1-rc0";
      copyToRoot = buildEnv {
        name = "image-root";
        pathsToLink = [ "/bin" ];
        paths = [ meshix-client ];
      };
      config = {
        Entrypoint = [ "/bin/meshix-client" ];
      };
    };

  clientWithStore =
    dockerTools.buildImageWithNixDb {
      fromImage = image-with-certs;
      name = "meshix-client-nix";
      tag = "0.0.1-rc0";
      copyToRoot = buildEnv {
        name = "image-root";
        pathsToLink = [ "/bin" ];
        paths = [
          meshix-client
          # nix-store uses cat program to display results as specified by
          # the image env variable NIX_PAGER.
          coreutils
          nix
          bash
        ];
      };
      runAsRoot = ''
        mkdir -p /etc/nix/
        touch /etc/nix/nix.conf
        echo "experimental-features = nix-command flakes" >> /etc/nix/nix.conf
      '';
      config = {
        Env = [
          "NIX_PAGER=cat"
          # A user is required by nix
          # https://github.com/NixOS/nix/blob/9348f9291e5d9e4ba3c4347ea1b235640f54fd79/src/libutil/util.cc#L478
          "USER=nobody"
        ];
        Entrypoint = [ "/bin/meshix-client" ];
      };
    };
}
