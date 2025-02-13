{ buildGoModule
, lib
, callPackage
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
      "server/go.mod"
      "server/go.sum"
      "gen/**"
      "go.*"
    ];
  };
  env.CGO_ENABLED = 0;
  version = "0.0.1";

  proxyVendor = true;
  subPackages = [
    "client/cmd"
  ];

  preBuild = ''
    mkdir -p gen/proto
    cp -r ${protobufGenerated}/* gen/proto
  '';

  postInstall = ''
    mv $out/bin/cmd $out/bin/meshix-client
  '';

  vendorHash = "sha256-ri4rbFcs70T9q617GWZFTwfiaAmgEPhISRV9HMhbXJs=";

  meta = {
    mainProgram = "meshix-client";
  };
}

