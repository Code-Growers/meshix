{ dockerTools
, stdenvNoCC
, meshix-server
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
in
dockerTools.streamLayeredImage {
  name = "meshix-server";
  tag = "0.0.1-rc0";
  contents = [
    configurations
    migrations
  ];
  config = {
    Cmd = [ "${meshix-server}/bin/${meshix-server.meta.mainProgram}" ];
  };
}

