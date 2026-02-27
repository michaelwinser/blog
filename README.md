# Blog

A minimal static site generator written in Go. Converts Markdown posts into a clean HTML blog with an index, archive, and RSS feed.

## Install

```bash
go install github.com/michaelwinser/blog@latest
```

## Quick Start

```bash
blog init my-blog
cd my-blog
blog new --title "Hello World"
blog generate
blog serve
# Open http://localhost:8080
```

## Commands

| Command | Description |
|---------|-------------|
| `init <path>` | Scaffold a new blog with templates, styles, and config |
| `new` | Create a new post (see below) |
| `generate` | Generate the static site into `docs/` |
| `serve` | Serve the site at `http://localhost:$PORT` (default 8080) |
| `clean` | Remove generated output |

### Creating Posts

```bash
# Basic post
blog new --title "My Post"

# With metadata
blog new --title "My Post" --date 2026-01-15 --description "A summary"

# Import from a URL (fetches HTML, converts via pandoc)
blog new --title "Imported Post" --url https://example.com/article

# Import from clipboard HTML (macOS)
pbpaste | blog new --stdin --title "Pasted Post"
```

Posts are Markdown files with YAML frontmatter stored in `posts/`:

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
my-blog/
├── site.yml                # Site configuration
├── posts/          # Markdown source files
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

## Deployment

The generator outputs to `docs/` with a `.nojekyll` marker, ready for GitHub Pages. Point your repository's Pages config at the `docs/` directory.

## Dependencies

- Go 1.23+
- `pandoc` and `curl` (only for HTML import features)
