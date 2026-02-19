---
title: "Hello World"
date: 2026-02-19
description: "My first post on this blog"
draft: false
---

Welcome to my blog! This is a simple static site generated with a custom Go-based tool.

## Why a custom SSG?

There are many great static site generators out there — Hugo, Jekyll, Eleventy — but sometimes it's nice to build something minimal that does exactly what you need. This generator is about 400 lines of Go and supports:

- Markdown content with YAML front matter
- A clean, responsive design with no JavaScript
- RSS feed for subscribers
- Archive page grouped by year

## How it works

Write a markdown file in `content/posts/`, run `make generate`, and the site appears in `docs/` ready for GitHub Pages.

```go
// That's it. Simple.
func main() {
    if err := run(); err != nil {
        fmt.Fprintf(os.Stderr, "error: %v\n", err)
        os.Exit(1)
    }
}
```

More posts coming soon!
