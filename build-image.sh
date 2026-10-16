#!/bin/bash
# Build and push san8888/emaki for linux/amd64 + linux/arm64 without a Docker daemon.
# Needs go and crane; DOCKER_CONFIG must hold Docker Hub credentials.
set -euo pipefail
R=index.docker.io/san8888/emaki; V=${1:-0.1.0}
rm -rf dist
for a in amd64 arm64; do
  mkdir -p dist/$a/root/etc/ssl/certs
  CGO_ENABLED=0 GOOS=linux GOARCH=$a go build -trimpath -ldflags "-s -w" -o dist/$a/root/emaki .
  cp /etc/ssl/cert.pem dist/$a/root/etc/ssl/certs/ca-certificates.crt
  (cd dist/$a/root && COPYFILE_DISABLE=1 tar --no-xattrs --no-mac-metadata --uid 0 --gid 0 --format ustar -cf ../layer.tar emaki etc)
  crane append --platform linux/$a -f dist/$a/layer.tar -t "${R}:$a"
  crane mutate --entrypoint=/emaki --set-platform linux/$a -e SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt -t "${R}:$a" "${R}:$a"
done
crane index append -m "${R}:amd64" -m "${R}:arm64" -t "${R}:$V"
crane copy "${R}:$V" "${R}:latest"
