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
in
generateProtobufCode {
  pname = "meshix";
  nativeBuildInputs = [
    protoc-gen-go
    protoc-gen-go-grpc
  ];
  outputPath = "gen/proto";
  hash = "sha256-y2AsT6s+NVWuxybI2FRJOQcDvpif7h2qm/0yKVcES4E=";
}
