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
  name = "meshix-server";
  src = lib.fileset.toSource {
    root = ./..;
    fileset = globset.lib.globs ./.. [
      "server/**"
      "agent/go.*"
      "client/go.*"
      "gen/**"
      "go.*"
    ];
  };
  env.CGO_ENABLED = 0;
  version = "0.0.1";
  gitSha = "S9smJTcfEAFIMEPeaPC1yOyO6QDHwFthOztf4";


  proxyVendor = true;
  subPackages = [
    "server/cmd"
  ];

  nativeBuildInputs = [
    sqlc
  ];

  preBuild = ''
    mkdir -p gen/proto
    cp -r ${protobufGenerated}/* gen/proto

    go generate ./server/cmd/main.go
  '';

  postInstall = ''
    mv $out/bin/cmd $out/bin/meshix-server
  '';

  vendorHash = "sha256-hNsQnAVrq5PXq/KpNa9U1Gz7LsVFo/B692DOCuueGWQ=";

  meta = {
    mainProgram = "meshix-server";
  };
}

