#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"

case "${1:-}" in
  build)
    docker compose build
    ;;
  generate)
    docker compose run --rm generate
    ;;
  clean)
    rm -rf docs
    ;;
  serve)
    cd docs && python3 -m http.server 8000
    ;;
  *)
    echo "Usage: $0 {build|generate|clean|serve}"
    exit 1
    ;;
esac
