!#/usr/bin/env sh

drv=$(nix build .#kiosk_app --json | jq -r '.[].outputs | to_entries[].value')

nix copy --to http://localhost:8088/cache $drv

echo $drv
