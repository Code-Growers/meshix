{ buildGoModule
, lib
, callPackage
, sqlc
, globset
}:
let
  protobufGenerated = callPackage ./common.nix {
    globset = globset;
  };
in
buildGoModule {
  name = "meshix-client";
  src = lib.fileset.toSource {
    root = ./..;
    fileset = globset.lib.globs ./.. [
      "client/**"
      "server/go.*"
      "agent/go.*"
      "gen/**"
      "go.*"
    ];
  };
  version = "0.0.1";
  gitSha = "S9smJTcfEAFIMEPeaPC1yOyO6QDHwFthOztf4";


  proxyVendor = true;
  subPackages = [
    "client/cmd"
  ];

  nativeBuildInputs = [
    sqlc
  ];

  preBuild = ''
    mkdir -p gen/proto
    cp -r ${protobufGenerated}/* gen/proto
  '';

  postInstall = ''
    mv $out/bin/cmd $out/bin/meshix-client
  '';

  vendorHash = "sha256-hNsQnAVrq5PXq/KpNa9U1Gz7LsVFo/B692DOCuueGWQ=";

  meta = {
    mainProgram = "meshix-client";
  };
}

