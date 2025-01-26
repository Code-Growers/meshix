!#/usr/bin/env sh

nix eval --json '.#kiosk_app.meta' | jq ".mainProgram"
