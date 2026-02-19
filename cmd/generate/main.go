package main

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/yuin/goldmark"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/parser"
)

const (
	contentDir  = "content/posts"
	templateDir = "templates"
	staticDir   = "static"
	outputDir   = "docs"
	siteURL     = "https://michaelw.github.io/blog"
	siteTitle   = "Michael's Blog"
	siteDesc    = "Thoughts on software, technology, and more"
	postsPerPage = 5
	postsInFeed  = 20
)

type Post struct {
	Title       string
	Slug        string
	Date        time.Time
	Description string
	Content     template.HTML
	URL         string
}

type HomePage struct {
	Posts []*Post
}

type PostPage struct {
	Post *Post
}

type ArchivePage struct {
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
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Step 1: Clean output directory contents (not the dir itself, it may be a mount)
	if err := cleanDir(outputDir); err != nil {
		return fmt.Errorf("cleaning output dir: %w", err)
	}

	// Step 2: Parse templates
	tmpl, err := parseTemplates()
	if err != nil {
		return fmt.Errorf("parsing templates: %w", err)
	}

	// Step 3: Parse markdown posts
	posts, err := parsePosts()
	if err != nil {
		return fmt.Errorf("parsing posts: %w", err)
	}

	// Step 4: Sort posts by date descending
	sort.Slice(posts, func(i, j int) bool {
		return posts[i].Date.After(posts[j].Date)
	})

	fmt.Printf("Found %d posts\n", len(posts))

	// Step 5: Generate pages
	if err := generatePostPages(tmpl, posts); err != nil {
		return fmt.Errorf("generating post pages: %w", err)
	}

	if err := generateHomePage(tmpl, posts); err != nil {
		return fmt.Errorf("generating home page: %w", err)
	}

	if err := generateArchivePage(tmpl, posts); err != nil {
		return fmt.Errorf("generating archive page: %w", err)
	}

	if err := generateRSSFeed(posts); err != nil {
		return fmt.Errorf("generating RSS feed: %w", err)
	}

	// Step 6: Copy static files
	if err := copyStaticFiles(); err != nil {
		return fmt.Errorf("copying static files: %w", err)
	}

	// Step 7: Write .nojekyll
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

	// Check draft status
	if draft, ok := metaData["draft"]; ok {
		if d, ok := draft.(bool); ok && d {
			fmt.Printf("Skipping draft: %s\n", filename)
			return nil, nil
		}
	}

	// Extract title
	title, _ := metaData["title"].(string)
	if title == "" {
		title = "Untitled"
	}

	// Extract description
	description, _ := metaData["description"].(string)

	// Extract date
	var date time.Time
	if d, ok := metaData["date"].(string); ok {
		date, err = time.Parse("2006-01-02", d)
		if err != nil {
			return nil, fmt.Errorf("parsing date %q: %w", d, err)
		}
	} else if d, ok := metaData["date"].(time.Time); ok {
		date = d
	}

	// Derive slug from filename: 2026-02-19-hello-world.md → hello-world
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
	// Remove date prefix (YYYY-MM-DD-)
	if len(name) > 11 && name[4] == '-' && name[7] == '-' && name[10] == '-' {
		name = name[11:]
	}
	return name
}

func generatePostPages(templates map[string]*template.Template, posts []*Post) error {
	for _, post := range posts {
		dir := filepath.Join(outputDir, "posts", post.Slug)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}

		f, err := os.Create(filepath.Join(dir, "index.html"))
		if err != nil {
			return err
		}

		err = templates["post.html"].Execute(f, PostPage{Post: post})
		f.Close()
		if err != nil {
			return fmt.Errorf("executing post template for %s: %w", post.Slug, err)
		}

		fmt.Printf("Generated: posts/%s/index.html\n", post.Slug)
	}
	return nil
}

func generateHomePage(templates map[string]*template.Template, posts []*Post) error {
	recent := posts
	if len(recent) > postsPerPage {
		recent = recent[:postsPerPage]
	}

	f, err := os.Create(filepath.Join(outputDir, "index.html"))
	if err != nil {
		return err
	}
	defer f.Close()

	if err := templates["home.html"].Execute(f, HomePage{Posts: recent}); err != nil {
		return fmt.Errorf("executing home template: %w", err)
	}

	fmt.Println("Generated: index.html")
	return nil
}

func generateArchivePage(templates map[string]*template.Template, posts []*Post) error {
	// Group posts by year
	yearMap := make(map[int][]*Post)
	for _, post := range posts {
		year := post.Date.Year()
		yearMap[year] = append(yearMap[year], post)
	}

	// Sort years descending
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

	if err := templates["archive.html"].Execute(f, ArchivePage{Years: years}); err != nil {
		return fmt.Errorf("executing archive template: %w", err)
	}

	fmt.Println("Generated: archive/index.html")
	return nil
}

func generateRSSFeed(posts []*Post) error {
	feedPosts := posts
	if len(feedPosts) > postsInFeed {
		feedPosts = feedPosts[:postsInFeed]
	}

	var items []RSSItem
	for _, post := range feedPosts {
		items = append(items, RSSItem{
			Title:       post.Title,
			Link:        siteURL + post.URL,
			Description: post.Description,
			PubDate:     post.Date.Format(time.RFC1123Z),
			GUID:        siteURL + post.URL,
		})
	}

	var lastBuild string
	if len(posts) > 0 {
		lastBuild = posts[0].Date.Format(time.RFC1123Z)
	}

	feed := RSSFeed{
		Version: "2.0",
		Channel: RSSChannel{
			Title:       siteTitle,
			Link:        siteURL,
			Description: siteDesc,
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
	return filepath.WalkDir(staticDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(staticDir, path)
		if err != nil {
			return err
		}

		destPath := filepath.Join(outputDir, relPath)

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
