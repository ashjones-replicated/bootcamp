#!/bin/sh
set -eu

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

echo "Cleaning up old chart archives..."
rm -f "$SCRIPT_DIR"/*.tgz

echo "Packaging bootcamp Helm chart..."
helm package "$SCRIPT_DIR/../helm" --destination "$SCRIPT_DIR"

echo "Downloading cloudnative-pg chart..."
helm pull cloudnative-pg \
  --repo https://cloudnative-pg.github.io/charts \
  --version 0.22.0 \
  --destination "$SCRIPT_DIR"

echo "Downloading traefik chart..."
helm pull traefik \
  --repo https://helm.traefik.io/traefik \
  --version 33.0.0 \
  --destination "$SCRIPT_DIR"

echo "Done. Release artifacts in $SCRIPT_DIR:"
ls "$SCRIPT_DIR"/*.tgz
