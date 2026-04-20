#!/bin/sh
set -e

UPLOAD_ADMIN_TOKEN="${UPLOAD_ADMIN_TOKEN:-devtoken}"

podman rm -f bootcamp-pg bootcamp-upload bootcamp-web 2>/dev/null || true
podman network rm bootcamp 2>/dev/null || true
podman network create bootcamp

podman build -t bootcamp-web "$(dirname "$0")/web"

podman run -d --network bootcamp --name bootcamp-pg \
    -e POSTGRES_USER=bootcamp \
    -e POSTGRES_PASSWORD=bootcamp \
    -e POSTGRES_DB=bootcamp \
    docker.io/library/postgres:16-alpine

podman run -d --network bootcamp --name bootcamp-upload \
    -e UPLOAD_BINDADDRESS=:8080 \
    -e UPLOAD_DATAPATH=/data \
    -e UPLOAD_PRESETADMINPASSWORD="$UPLOAD_ADMIN_TOKEN" \
    --tmpfs /data \
    codeberg.org/sparrow/upload:v2.0.0

until podman exec bootcamp-pg pg_isready -U bootcamp -q 2>/dev/null; do sleep 1; done
until podman logs bootcamp-upload 2>&1 | grep -q "listening on"; do sleep 1; done

podman run --network bootcamp --name bootcamp-web \
    -e DATABASE_URL="postgres://bootcamp:bootcamp@bootcamp-pg:5432/bootcamp?sslmode=disable" \
    -e UPLOAD_SERVICE_URL="http://bootcamp-upload:8080" \
    -e UPLOAD_ADMIN_TOKEN="$UPLOAD_ADMIN_TOKEN" \
    -e BIND_ADDRESS=":8081" \
    -e COOKIE_SECURE=false \
    -e ALLOW_PRIVATE_UPLOADS="${ALLOW_PRIVATE_UPLOADS:-true}" \
    -e ALLOW_SINGLE_USE_LINKS="${ALLOW_SINGLE_USE_LINKS:-true}" \
    -p 8081:8081 \
    bootcamp-web
