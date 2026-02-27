package main

import (
	"bytes"
	"encoding/xml"
	"flag"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/yuin/goldmark"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/parser"
	"gopkg.in/yaml.v2"
)

const (
	contentDir   = "posts"
	templateDir  = "templates"
	staticDir    = "static"
	outputDir    = "docs"
	postsPerPage = 5
	postsInFeed  = 20
)

type SiteConfig struct {
	Title       string `yaml:"title"`
	URL         string `yaml:"url"`
	Description string `yaml:"description"`
}

type Post struct {
	Title       string
	Slug        string
	Date        time.Time
	Description string
	Content     template.HTML
	URL         string
}

type HomePage struct {
	Site  SiteConfig
	Posts []*Post
}

type PostPage struct {
	Site SiteConfig
	Post *Post
}

type ArchivePage struct {
	Site  SiteConfig
	Years []YearGroup
}

type YearGroup struct {
	Year  int
	Posts []*Post
}

type RSSFeed struct {
	XMLName xml.Name   `xml:"rss"`
	Version string     `xml:"version,attr"`
	Channel RSSChannel `xml:"channel"`
}

type RSSChannel struct {
	Title       string    `xml:"title"`
	Link        string    `xml:"link"`
	Description string    `xml:"description"`
	LastBuild   string    `xml:"lastBuildDate"`
	Items       []RSSItem `xml:"item"`
}

type RSSItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
	GUID        string `xml:"guid"`
}

func main() {
	cmd := "generate"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}

	var err error
	switch cmd {
	case "generate":
		err = runGenerate()
	case "serve":
		err = runServe()
	case "clean":
		err = runClean()
	case "new":
		err = runNew(os.Args[2:])
	case "init":
		err = runInit(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		fmt.Fprintf(os.Stderr, "Usage: blog {generate|serve|clean|new|init}\n")
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func runServe() error {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("Serving %s on http://0.0.0.0:%s\n", outputDir, port)
	return http.ListenAndServe(":"+port, http.FileServer(http.Dir(outputDir)))
}

func runClean() error {
	if err := cleanDir(outputDir); err != nil {
		return fmt.Errorf("cleaning output dir: %w", err)
	}
	fmt.Println("Cleaned output directory")
	return nil
}

func runNew(args []string) error {
	fs := flag.NewFlagSet("new", flag.ExitOnError)
	title := fs.String("title", "", "Post title (required)")
	date := fs.String("date", time.Now().Format("2006-01-02"), "Post date (YYYY-MM-DD)")
	description := fs.String("description", "", "Post description")
	url := fs.String("url", "", "Fetch URL and convert HTML via pandoc")
	stdin := fs.Bool("stdin", false, "Read HTML from stdin and convert via pandoc")
	fs.Parse(args)

	if *title == "" {
		return fmt.Errorf("--title is required")
	}

	slug := slugify(*title)
	filename := fmt.Sprintf("%s-%s.md", *date, slug)
	filepath := filepath.Join(contentDir, filename)

	var body string
	var err error

	if *url != "" {
		body, err = convertURL(*url)
		if err != nil {
			return fmt.Errorf("converting URL: %w", err)
		}
	} else if *stdin {
		body, err = convertStdin()
		if err != nil {
			return fmt.Errorf("converting stdin: %w", err)
		}
	}

	var buf bytes.Buffer
	buf.WriteString("---\n")
	fmt.Fprintf(&buf, "title: \"%s\"\n", *title)
	fmt.Fprintf(&buf, "date: %s\n", *date)
	if *description != "" {
		fmt.Fprintf(&buf, "description: \"%s\"\n", *description)
	}
	buf.WriteString("---\n")
	if body != "" {
		buf.WriteString("\n")
		buf.WriteString(body)
		buf.WriteString("\n")
	}

	if err := os.MkdirAll(contentDir, 0o755); err != nil {
		return fmt.Errorf("creating content dir: %w", err)
	}

	if err := os.WriteFile(filepath, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("writing post: %w", err)
	}

	fmt.Println(filepath)
	return nil
}

func runInit(args []string) error {
	target := "."
	if len(args) > 0 {
		target = args[0]
	}

	if info, err := os.Stat(target); err == nil {
		if !info.IsDir() {
			return fmt.Errorf("%s exists and is not a directory", target)
		}
		entries, err := os.ReadDir(target)
		if err != nil {
			return fmt.Errorf("reading %s: %w", target, err)
		}
		for _, e := range entries {
			name := e.Name()
			if name == ".git" || name == ".DS_Store" {
				continue
			}
			return fmt.Errorf("%s is not empty", target)
		}
	}

	// Create directory structure
	dirs := []string{
		filepath.Join(target, "posts"),
		filepath.Join(target, "docs"),
		filepath.Join(target, "templates"),
		filepath.Join(target, "static", "css"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("creating %s: %w", dir, err)
		}
	}

	// site.yml
	siteYml := `title: "My Blog"
url: "https://example.com"
description: "A blog about things"
`
	if err := os.WriteFile(filepath.Join(target, "site.yml"), []byte(siteYml), 0o644); err != nil {
		return err
	}

	// .gitkeep files
	for _, path := range []string{
		filepath.Join(target, "posts", ".gitkeep"),
		filepath.Join(target, "docs", ".gitkeep"),
	} {
		if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
			return err
		}
	}

	// .gitignore
	gitignore := `.env
.DS_Store
`
	if err := os.WriteFile(filepath.Join(target, ".gitignore"), []byte(gitignore), 0o644); err != nil {
		return err
	}

	// templates/base.html
	baseHTML := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>{{block "title" .}}{{.Site.Title}}{{end}}</title>
    <link rel="stylesheet" href="/css/style.css">
    <link rel="alternate" type="application/rss+xml" title="RSS Feed" href="/feed.xml">
</head>
<body>
    <header>
        <nav>
            <a href="/" class="site-title">{{.Site.Title}}</a>
            <div class="nav-links">
                <a href="/">Home</a>
                <a href="/archive/">Archive</a>
                <a href="/feed.xml">RSS</a>
            </div>
        </nav>
    </header>
    <main>
        {{block "content" .}}{{end}}
    </main>
    <footer>
        <p>&copy; {{.Site.Title}}</p>
    </footer>
</body>
</html>
`
	if err := os.WriteFile(filepath.Join(target, "templates", "base.html"), []byte(baseHTML), 0o644); err != nil {
		return err
	}

	// templates/home.html
	homeHTML := `{{define "title"}}{{.Site.Title}}{{end}}
{{define "content"}}
<h1>Recent Posts</h1>
{{range .Posts}}
<article class="post-summary">
    <h2><a href="{{.URL}}">{{.Title}}</a></h2>
    <time datetime="{{.Date.Format "2006-01-02"}}">{{formatDate .Date}}</time>
    {{if .Description}}<p>{{.Description}}</p>{{end}}
</article>
{{else}}
<p>No posts yet.</p>
{{end}}
{{end}}
`
	if err := os.WriteFile(filepath.Join(target, "templates", "home.html"), []byte(homeHTML), 0o644); err != nil {
		return err
	}

	// templates/post.html
	postHTML := `{{define "title"}}{{.Post.Title}} — {{.Site.Title}}{{end}}
{{define "content"}}
<article class="post">
    <header class="post-header">
        <h1>{{.Post.Title}}</h1>
        <time datetime="{{.Post.Date.Format "2006-01-02"}}">{{formatDate .Post.Date}}</time>
    </header>
    <div class="post-content">
        {{.Post.Content}}
    </div>
</article>
{{end}}
`
	if err := os.WriteFile(filepath.Join(target, "templates", "post.html"), []byte(postHTML), 0o644); err != nil {
		return err
	}

	// templates/archive.html
	archiveHTML := `{{define "title"}}Archive — {{.Site.Title}}{{end}}
{{define "content"}}
<h1>Archive</h1>
{{range .Years}}
<section class="archive-year">
    <h2>{{.Year}}</h2>
    <ul>
        {{range .Posts}}
        <li>
            <time datetime="{{.Date.Format "2006-01-02"}}">{{formatDateShort .Date}}</time>
            <a href="{{.URL}}">{{.Title}}</a>
        </li>
        {{end}}
    </ul>
</section>
{{end}}
{{end}}
`
	if err := os.WriteFile(filepath.Join(target, "templates", "archive.html"), []byte(archiveHTML), 0o644); err != nil {
		return err
	}

	// static/css/style.css
	styleCSS := `*,
*::before,
*::after {
    box-sizing: border-box;
    margin: 0;
    padding: 0;
}

body {
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Oxygen, sans-serif;
    line-height: 1.6;
    color: #333;
    max-width: 42rem;
    margin: 0 auto;
    padding: 2rem 1rem;
}

header nav {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 3rem;
    padding-bottom: 1rem;
    border-bottom: 1px solid #eee;
}

.site-title {
    font-weight: 700;
    font-size: 1.2rem;
    text-decoration: none;
    color: #111;
}

.nav-links a {
    margin-left: 1.5rem;
    text-decoration: none;
    color: #555;
}

.nav-links a:hover {
    color: #111;
}

h1 { font-size: 1.8rem; margin-bottom: 1rem; }
h2 { font-size: 1.4rem; margin-bottom: 0.5rem; }

a { color: #0066cc; }
a:hover { color: #004499; }

.post-summary {
    margin-bottom: 2rem;
}

.post-summary h2 { margin-bottom: 0.25rem; }
.post-summary time { color: #888; font-size: 0.9rem; }
.post-summary p { margin-top: 0.5rem; color: #555; }

.post-header { margin-bottom: 2rem; }
.post-header time { color: #888; font-size: 0.9rem; }

.post-content h2 { margin-top: 2rem; }
.post-content h3 { margin-top: 1.5rem; font-size: 1.2rem; }
.post-content p { margin-top: 1rem; }
.post-content ul,
.post-content ol { margin-top: 1rem; padding-left: 1.5rem; }
.post-content li { margin-top: 0.25rem; }

.post-content pre {
    margin-top: 1rem;
    padding: 1rem;
    background: #f5f5f5;
    border-radius: 4px;
    overflow-x: auto;
    font-size: 0.9rem;
    line-height: 1.5;
}

.post-content code {
    font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
    font-size: 0.9em;
}

.post-content :not(pre) > code {
    background: #f5f5f5;
    padding: 0.15em 0.3em;
    border-radius: 3px;
}

.archive-year { margin-bottom: 2rem; }
.archive-year ul { list-style: none; padding: 0; }
.archive-year li { margin-top: 0.5rem; }
.archive-year time {
    display: inline-block;
    width: 4rem;
    color: #888;
    font-size: 0.9rem;
}

footer {
    margin-top: 4rem;
    padding-top: 1rem;
    border-top: 1px solid #eee;
    color: #888;
    font-size: 0.85rem;
}
`
	if err := os.WriteFile(filepath.Join(target, "static", "css", "style.css"), []byte(styleCSS), 0o644); err != nil {
		return err
	}

	fmt.Printf("Created blog at %s\n", target)
	fmt.Println()
	fmt.Println("Next steps:")
	if target != "." {
		fmt.Printf("  cd %s\n", target)
	}
	fmt.Println("  Edit site.yml with your blog details")
	fmt.Println("  blog new --title \"My First Post\"")
	fmt.Println("  blog generate")
	fmt.Println("  blog serve")

	return nil
}

func slugify(s string) string {
	s = strings.ToLower(s)
	re := regexp.MustCompile(`[^a-z0-9]+`)
	s = re.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}

func convertURL(url string) (string, error) {
	curl := exec.Command("curl", "-sL", url)
	pandoc := exec.Command("pandoc", "-f", "html", "-t", "markdown-fenced_divs-bracketed_spans-header_attributes-link_attributes", "--wrap=none")

	pipe, err := curl.StdoutPipe()
	if err != nil {
		return "", err
	}
	pandoc.Stdin = pipe

	var out bytes.Buffer
	pandoc.Stdout = &out
	pandoc.Stderr = os.Stderr

	if err := curl.Start(); err != nil {
		return "", fmt.Errorf("starting curl: %w", err)
	}
	if err := pandoc.Start(); err != nil {
		return "", fmt.Errorf("starting pandoc: %w", err)
	}

	if err := curl.Wait(); err != nil {
		return "", fmt.Errorf("curl failed: %w", err)
	}
	if err := pandoc.Wait(); err != nil {
		return "", fmt.Errorf("pandoc failed: %w", err)
	}

	return out.String(), nil
}

func convertStdin() (string, error) {
	cmd := exec.Command("pandoc", "-f", "html", "-t", "markdown-fenced_divs-bracketed_spans-header_attributes-link_attributes", "--wrap=none")
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr

	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("pandoc failed: %w", err)
	}
	return string(out), nil
}

func loadConfig() (SiteConfig, error) {
	data, err := os.ReadFile("site.yml")
	if err != nil {
		return SiteConfig{}, fmt.Errorf("reading site.yml: %w", err)
	}

	var cfg SiteConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return SiteConfig{}, fmt.Errorf("parsing site.yml: %w", err)
	}

	if cfg.Title == "" {
		cfg.Title = "My Blog"
	}
	if cfg.URL == "" {
		cfg.URL = "https://example.com"
	}

	return cfg, nil
}

func runGenerate() error {
	site, err := loadConfig()
	if err != nil {
		return err
	}

	if err := cleanDir(outputDir); err != nil {
		return fmt.Errorf("cleaning output dir: %w", err)
	}

	tmpl, err := parseTemplates()
	if err != nil {
		return fmt.Errorf("parsing templates: %w", err)
	}

	posts, err := parsePosts()
	if err != nil {
		return fmt.Errorf("parsing posts: %w", err)
	}

	sort.Slice(posts, func(i, j int) bool {
		return posts[i].Date.After(posts[j].Date)
	})

	fmt.Printf("Found %d posts\n", len(posts))

	if err := generatePostPages(tmpl, site, posts); err != nil {
		return fmt.Errorf("generating post pages: %w", err)
	}

	if err := generateHomePage(tmpl, site, posts); err != nil {
		return fmt.Errorf("generating home page: %w", err)
	}

	if err := generateArchivePage(tmpl, site, posts); err != nil {
		return fmt.Errorf("generating archive page: %w", err)
	}

	if err := generateRSSFeed(site, posts); err != nil {
		return fmt.Errorf("generating RSS feed: %w", err)
	}

	if err := copyStaticFiles(); err != nil {
		return fmt.Errorf("copying static files: %w", err)
	}

	if err := os.WriteFile(filepath.Join(outputDir, ".nojekyll"), []byte{}, 0o644); err != nil {
		return fmt.Errorf("writing .nojekyll: %w", err)
	}

	fmt.Println("Site generated successfully!")
	return nil
}

func parseTemplates() (map[string]*template.Template, error) {
	funcMap := template.FuncMap{
		"formatDate": func(t time.Time) string {
			return t.Format("January 2, 2006")
		},
		"formatDateShort": func(t time.Time) string {
			return t.Format("Jan 2")
		},
	}

	pages := []string{"home.html", "post.html", "archive.html"}
	templates := make(map[string]*template.Template, len(pages))

	baseFile := filepath.Join(templateDir, "base.html")

	for _, page := range pages {
		pageFile := filepath.Join(templateDir, page)
		t, err := template.New("base.html").Funcs(funcMap).ParseFiles(baseFile, pageFile)
		if err != nil {
			return nil, fmt.Errorf("parsing %s: %w", page, err)
		}
		templates[page] = t
	}

	return templates, nil
}

func parsePosts() ([]*Post, error) {
	md := goldmark.New(
		goldmark.WithExtensions(
			meta.Meta,
		),
	)

	var posts []*Post

	entries, err := os.ReadDir(contentDir)
	if err != nil {
		return nil, fmt.Errorf("reading content dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		post, err := parsePost(md, entry.Name())
		if err != nil {
			return nil, fmt.Errorf("parsing %s: %w", entry.Name(), err)
		}
		if post != nil {
			posts = append(posts, post)
		}
	}

	return posts, nil
}

func parsePost(md goldmark.Markdown, filename string) (*Post, error) {
	source, err := os.ReadFile(filepath.Join(contentDir, filename))
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	ctx := parser.NewContext()
	if err := md.Convert(source, &buf, parser.WithContext(ctx)); err != nil {
		return nil, fmt.Errorf("converting markdown: %w", err)
	}

	metaData := meta.Get(ctx)

	if draft, ok := metaData["draft"]; ok {
		if d, ok := draft.(bool); ok && d {
			fmt.Printf("Skipping draft: %s\n", filename)
			return nil, nil
		}
	}

	title, _ := metaData["title"].(string)
	if title == "" {
		title = "Untitled"
	}

	description, _ := metaData["description"].(string)

	var date time.Time
	if d, ok := metaData["date"].(string); ok {
		date, err = time.Parse("2006-01-02", d)
		if err != nil {
			return nil, fmt.Errorf("parsing date %q: %w", d, err)
		}
	} else if d, ok := metaData["date"].(time.Time); ok {
		date = d
	}

	slug := deriveSlug(filename)

	return &Post{
		Title:       title,
		Slug:        slug,
		Date:        date,
		Description: description,
		Content:     template.HTML(buf.String()),
		URL:         "/posts/" + slug + "/",
	}, nil
}

func deriveSlug(filename string) string {
	name := strings.TrimSuffix(filename, ".md")
	if len(name) > 11 && name[4] == '-' && name[7] == '-' && name[10] == '-' {
		name = name[11:]
	}
	return name
}

func generatePostPages(templates map[string]*template.Template, site SiteConfig, posts []*Post) error {
	for _, post := range posts {
		dir := filepath.Join(outputDir, "posts", post.Slug)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}

		f, err := os.Create(filepath.Join(dir, "index.html"))
		if err != nil {
			return err
		}

		err = templates["post.html"].Execute(f, PostPage{Site: site, Post: post})
		f.Close()
		if err != nil {
			return fmt.Errorf("executing post template for %s: %w", post.Slug, err)
		}

		fmt.Printf("Generated: posts/%s/index.html\n", post.Slug)
	}
	return nil
}

func generateHomePage(templates map[string]*template.Template, site SiteConfig, posts []*Post) error {
	recent := posts
	if len(recent) > postsPerPage {
		recent = recent[:postsPerPage]
	}

	f, err := os.Create(filepath.Join(outputDir, "index.html"))
	if err != nil {
		return err
	}
	defer f.Close()

	if err := templates["home.html"].Execute(f, HomePage{Site: site, Posts: recent}); err != nil {
		return fmt.Errorf("executing home template: %w", err)
	}

	fmt.Println("Generated: index.html")
	return nil
}

func generateArchivePage(templates map[string]*template.Template, site SiteConfig, posts []*Post) error {
	yearMap := make(map[int][]*Post)
	for _, post := range posts {
		year := post.Date.Year()
		yearMap[year] = append(yearMap[year], post)
	}

	var years []YearGroup
	for year, yearPosts := range yearMap {
		years = append(years, YearGroup{Year: year, Posts: yearPosts})
	}
	sort.Slice(years, func(i, j int) bool {
		return years[i].Year > years[j].Year
	})

	dir := filepath.Join(outputDir, "archive")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	f, err := os.Create(filepath.Join(dir, "index.html"))
	if err != nil {
		return err
	}
	defer f.Close()

	if err := templates["archive.html"].Execute(f, ArchivePage{Site: site, Years: years}); err != nil {
		return fmt.Errorf("executing archive template: %w", err)
	}

	fmt.Println("Generated: archive/index.html")
	return nil
}

func generateRSSFeed(site SiteConfig, posts []*Post) error {
	feedPosts := posts
	if len(feedPosts) > postsInFeed {
		feedPosts = feedPosts[:postsInFeed]
	}

	var items []RSSItem
	for _, post := range feedPosts {
		items = append(items, RSSItem{
			Title:       post.Title,
			Link:        site.URL + post.URL,
			Description: post.Description,
			PubDate:     post.Date.Format(time.RFC1123Z),
			GUID:        site.URL + post.URL,
		})
	}

	var lastBuild string
	if len(posts) > 0 {
		lastBuild = posts[0].Date.Format(time.RFC1123Z)
	}

	feed := RSSFeed{
		Version: "2.0",
		Channel: RSSChannel{
			Title:       site.Title,
			Link:        site.URL,
			Description: site.Description,
			LastBuild:   lastBuild,
			Items:       items,
		},
	}

	f, err := os.Create(filepath.Join(outputDir, "feed.xml"))
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := f.WriteString(xml.Header); err != nil {
		return err
	}

	enc := xml.NewEncoder(f)
	enc.Indent("", "  ")
	if err := enc.Encode(feed); err != nil {
		return fmt.Errorf("encoding RSS: %w", err)
	}

	fmt.Println("Generated: feed.xml")
	return nil
}

func copyStaticFiles() error {
	if _, err := os.Stat(staticDir); err == nil {
		if err := copyDir(staticDir, outputDir); err != nil {
			return fmt.Errorf("copying static files: %w", err)
		}
	}
	return nil
}

func copyDir(srcDir, destBase string) error {
	return filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}

		destPath := filepath.Join(destBase, relPath)

		if d.IsDir() {
			return os.MkdirAll(destPath, 0o755)
		}

		return copyFile(path, destPath)
	})
}

func cleanDir(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := os.RemoveAll(filepath.Join(dir, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
