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
      "client/**"
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

  vendorHash = "sha256-ri4rbFcs70T9q617GWZFTwfiaAmgEPhISRV9HMhbXJs=";

  meta = {
    mainProgram = "meshix-server";
  };
}

