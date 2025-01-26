{ buildGoModule
, lib
, buf
, sqlc
, protoc-gen-go
, protoc-gen-go-grpc
, stdenv
, cacert
, globset
}:
let
  # Buf downloads dependencies from an external repo - there doesn't seem to
  # really be any good way around it. We'll use a fixed-output derivation so it
  # can download what it needs, and output the relevant generated code for use
  # during the main build.
  generateProtobufCode =
    { pname
    , nativeBuildInputs ? [ ]
    , bufArgs ? ""
    , workDir ? "."
    , outputPath
    , hash
    ,
    }:
    stdenv.mkDerivation {
      name = "${pname}-buf-generated";

      src = lib.fileset.toSource {
        root = ./..;
        fileset = globset.lib.globs ./.. [
          "proto/**"
          "gen/**"
          "buf.*"
        ];
      };

      nativeBuildInputs = nativeBuildInputs ++ [
        buf
        cacert
      ];

      buildPhase = ''
        cd ${workDir}
        HOME=$TMPDIR buf generate ${bufArgs}
      '';

      installPhase = ''
        cp -r ${outputPath} $out
      '';

      outputHashMode = "recursive";
      outputHashAlgo = "sha256";
      outputHash = hash;
    };

  protobufGenerated = generateProtobufCode {
    pname = "meshix";
    nativeBuildInputs = [
      protoc-gen-go
      protoc-gen-go-grpc
    ];
    outputPath = "gen/proto";
    hash = "sha256-vCXV0UXW7UaTk12bc2QViUIsUlr/Xw7+XDykmZkY2IY=";
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

  vendorHash = "sha256-8xyhN2Cfsgvz1VJAbBkYervvEhpjUQF8BL6k/Q8ViG8=";

  meta = {
    mainProgram = "meshix-server";
  };
}

