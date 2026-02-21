#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"

case "${1:-}" in
  build)
    docker compose build
    ;;
  generate)
    docker compose run --rm blog
    ;;
  serve)
    docker compose run --rm --service-ports blog serve
    ;;
  clean)
    docker compose run --rm blog clean
    ;;
  new)
    shift
    # Check for --html flag: pipe clipboard into container via --stdin
    args=()
    html_mode=false
    for arg in "$@"; do
      if [[ "$arg" == "--html" ]]; then
        html_mode=true
      else
        args+=("$arg")
      fi
    done
    if [[ "$html_mode" == true ]]; then
      pbpaste | docker compose run --rm -T blog new --stdin "${args[@]}"
    else
      docker compose run --rm blog new "${args[@]}"
    fi
    ;;
  init)
    target="${2:-}"
    if [[ -z "$target" ]]; then
      echo "Usage: $0 init <path>"
      echo "  Scaffold a new blog repository at the given path."
      exit 1
    fi

    # Resolve engine dir (where this script lives)
    engine_dir="$(cd "$(dirname "$0")" && pwd)"

    # Create target as absolute path
    if [[ "$target" != /* ]]; then
      target="$(pwd)/$target"
    fi

    if [[ -e "$target" ]]; then
      echo "Error: $target already exists"
      exit 1
    fi

    mkdir -p "$target/content/posts" "$target/docs"

    # Compute relative path from target to engine
    engine_rel="$(python3 -c "import os.path; print(os.path.relpath('$engine_dir', '$target'))")"

    # site.yml
    cat > "$target/site.yml" <<'SITE'
title: "My Blog"
url: "https://example.com"
description: "A blog about things"
SITE

    # docker-compose.yml
    cat > "$target/docker-compose.yml" <<COMPOSE
services:
  blog:
    build: ${engine_rel}
    volumes:
      - ./content:/site/content
      - ./static:/site/static:ro
      - ./site.yml:/site/site.yml:ro
      - ./docs:/site/docs
    ports:
      - "\${PORT:-9876}:8080"
COMPOSE

    # blog.sh (thin wrapper)
    cat > "$target/blog.sh" <<'WRAPPER'
#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"

case "${1:-}" in
  build)
    docker compose build
    ;;
  generate)
    docker compose run --rm blog
    ;;
  serve)
    docker compose run --rm --service-ports blog serve
    ;;
  clean)
    docker compose run --rm blog clean
    ;;
  new)
    shift
    # Check for --html flag: pipe clipboard into container via --stdin
    args=()
    html_mode=false
    for arg in "$@"; do
      if [[ "$arg" == "--html" ]]; then
        html_mode=true
      else
        args+=("$arg")
      fi
    done
    if [[ "$html_mode" == true ]]; then
      pbpaste | docker compose run --rm -T blog new --stdin "${args[@]}"
    else
      docker compose run --rm blog new "${args[@]}"
    fi
    ;;
  *)
    echo "Usage: $0 {build|generate|serve|clean|new}"
    echo ""
    echo "  build                        Build the container image"
    echo "  generate                     Generate the site"
    echo "  serve                        Serve at http://localhost:\${PORT:-9876}"
    echo "  clean                        Remove generated output"
    echo "  new --title <t> [options]    Create a new post"
    echo "    --date <d>                 Post date (default: today)"
    echo "    --description <d>          Post description"
    echo "    --url <url>                Fetch URL and convert HTML via pandoc"
    echo "    --html                     Convert clipboard HTML via pandoc"
    exit 1
    ;;
esac
WRAPPER
    chmod +x "$target/blog.sh"

    # .gitkeep files
    touch "$target/content/posts/.gitkeep"
    touch "$target/docs/.gitkeep"

    # .gitignore
    cat > "$target/.gitignore" <<'GITIGNORE'
.env
.DS_Store
GITIGNORE

    echo "Created blog scaffold at $target"
    echo ""
    echo "Files:"
    echo "  site.yml              - Edit with your blog title, URL, description"
    echo "  docker-compose.yml    - Docker Compose config (build: $engine_rel)"
    echo "  blog.sh               - CLI wrapper"
    echo "  content/posts/        - Write your posts here"
    echo "  docs/                 - Generated site output"
    echo "  .gitignore"
    echo ""
    echo "Next steps:"
    echo "  cd $target"
    echo "  Edit site.yml with your blog details"
    echo "  ./blog.sh build"
    echo "  ./blog.sh new --title \"My First Post\""
    echo "  ./blog.sh generate"
    echo "  ./blog.sh serve"
    ;;
  *)
    echo "Usage: $0 {build|generate|serve|clean|new|init}"
    echo ""
    echo "  build                        Build the container image"
    echo "  generate                     Generate the site"
    echo "  serve                        Serve at http://localhost:\${PORT:-9876}"
    echo "  clean                        Remove generated output"
    echo "  new --title <t> [options]    Create a new post"
    echo "    --date <d>                 Post date (default: today)"
    echo "    --description <d>          Post description"
    echo "    --url <url>                Fetch URL and convert HTML via pandoc"
    echo "    --html                     Convert clipboard HTML via pandoc"
    echo "  init <path>                  Scaffold a new blog repository"
    exit 1
    ;;
esac
