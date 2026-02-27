# Blog

A minimal static site generator written in Go. Converts Markdown posts into a clean HTML blog with an index, archive, and RSS feed. Runs entirely inside Docker.

## Quick Start

```bash
./blog.sh build
./blog.sh new --title "Hello World"
./blog.sh generate
./blog.sh serve
# Open http://localhost:${PORT:-9876}
```

## Commands

| Command | Description |
|---------|-------------|
| `build` | Build the Docker image |
| `generate` | Generate the static site into `docs/` |
| `serve` | Serve the site at `http://localhost:${PORT:-9876}` |
| `clean` | Remove generated output |
| `new` | Create a new post (see below) |
| `init <path>` | Scaffold a new blog repository |

### Creating Posts

```bash
# Basic post
./blog.sh new --title "My Post"

# With metadata
./blog.sh new --title "My Post" --date 2026-01-15 --description "A summary"

# Import from a URL (fetches HTML → converts via pandoc)
./blog.sh new --title "Imported Post" --url https://example.com/article

# Import from clipboard HTML (macOS)
./blog.sh new --title "Pasted Post" --html
```

Posts are Markdown files with YAML frontmatter stored in `content/posts/`:

```markdown
---
title: "My Post"
date: 2026-01-15
description: "Optional summary"
draft: false
---

Post content in Markdown.
```

Set `draft: true` to exclude a post from generation.

## Project Structure

```
blog/
├── cmd/generate/main.go    # Go application
├── blog.sh                 # CLI wrapper
├── site.yml                # Site configuration
├── Dockerfile              # Multi-stage build
├── docker-compose.yml      # Docker Compose config
├── content/posts/          # Markdown source files
├── templates/              # Go HTML templates
├── static/css/             # Stylesheet
└── docs/                   # Generated output
```

## Configuration

Edit `site.yml`:

```yaml
title: "My Blog"
url: "https://example.com"
description: "A blog about things"
```

## Starting a New Blog

The recommended way to create a blog is to **fork this repository**. You get a self-contained blog that you can customize freely, while still being able to pull upstream to pick up engine improvements.

```bash
# Fork on GitHub, then:
git clone <your-fork-url>
cd my-blog

# Configure
vim site.yml                          # Set title, URL, description
./blog.sh build
./blog.sh new --title "First Post"
./blog.sh generate
./blog.sh serve
```

To pick up engine updates later:

```bash
git remote add upstream <original-repo-url>
git fetch upstream
git merge upstream/main
```

Your content (`content/posts/`, `site.yml`) and any template/style customizations live alongside the engine, so merges are straightforward — upstream changes to the engine won't conflict with your posts.

### Local Multi-Blog Alternative

For running multiple blogs on the same machine during development, the `init` command scaffolds a lightweight blog repo that references the engine directory via a relative path:

```bash
./blog.sh init ~/projects/my-other-blog
```

This avoids duplicating the engine but requires the engine directory to be present locally.

## Deployment

The generator outputs to `docs/` with a `.nojekyll` marker, ready for GitHub Pages. Point your repository's Pages config at the `docs/` directory.

## Dependencies

- Docker and Docker Compose
- macOS `pbpaste` (only for `--html` clipboard import)
