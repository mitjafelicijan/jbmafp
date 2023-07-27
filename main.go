package main

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	yaml "gopkg.in/yaml.v3"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"

	"github.com/DavidBelicza/TextRank/v2"
	"github.com/alexflint/go-arg"
	"github.com/gosimple/slug"
	"github.com/microcosm-cc/bluemonday"

	"github.com/tdewolff/minify/v2"
	mcss "github.com/tdewolff/minify/v2/css"
	mhtml "github.com/tdewolff/minify/v2/html"
	mjs "github.com/tdewolff/minify/v2/js"

	highlighting "github.com/yuin/goldmark-highlighting/v2"

	cp "github.com/otiai10/copy"

	_ "embed"
)

type ConfigExtrasItem struct {
	Type     string `yaml:"type"`
	Template string `yaml:"template"`
	URL      string `yaml:"url"`
}

type Config struct {
	Title        string             `yaml:"title"`
	Description  string             `yaml:"description"`
	BaseURL      string             `yaml:"baseurl"`
	Language     string             `yaml:"language"`
	Highlighting string             `yaml:"highlighting"`
	Minify       bool               `yaml:"minify"`
	Extras       []ConfigExtrasItem `yaml:"extras"`
}

type Page struct {
	Filepath     string
	Raw          string
	HTML         template.HTML
	Text         string
	Summary      string
	Meta         map[string]interface{}
	Title        string
	Type         string
	RelPermalink string
	Created      time.Time
	Draft        bool
}

//go:embed "files/config.yaml"
var EmbedConfig string

//go:embed "files/first.md"
var EmbedPost string

//go:embed "files/base.html"
var EmbedTemplateBase string

//go:embed "files/index.html"
var EmbedTemplateIndex string

//go:embed "files/post.html"
var EmbedTemplatePost string

//go:embed "files/index.xml"
var EmbedTemplateFeed string

// Function to clean HTML tags using bluemonday.
func cleanHTMLTags(htmlString string) string {
	p := bluemonday.StrictPolicy()
	cleanString := p.Sanitize(htmlString)
	return cleanString
}

func simpleServer(projectRoot string) {
	fs := http.FileServer(http.Dir(path.Join(projectRoot, "public")))
	http.Handle("/", fs)
	log.Println("Server started on http://localhost:6969")
	log.Fatal(http.ListenAndServe(":6969", nil))
}

func initializeProject(projectRoot string) {
	log.Println("Initializing new project")

	if err := os.Mkdir(path.Join(projectRoot, "templates"), 0755); err != nil && !os.IsExist(err) {
		log.Println("Error creating directory:", err)
		return
	}

	if err := os.Mkdir(path.Join(projectRoot, "content"), 0755); err != nil && !os.IsExist(err) {
		log.Println("Error creating directory:", err)
		return
	}

	if err := os.Mkdir(path.Join(projectRoot, "static"), 0755); err != nil && !os.IsExist(err) {
		log.Println("Error creating directory:", err)
		return
	}

	os.WriteFile(path.Join(projectRoot, "templates", ".gitkeep"), []byte{}, 0755)
	os.WriteFile(path.Join(projectRoot, "content", ".gitkeep"), []byte{}, 0755)
	os.WriteFile(path.Join(projectRoot, "static", ".gitkeep"), []byte{}, 0755)

	os.WriteFile(path.Join(projectRoot, "config.yaml"), []byte(EmbedConfig), 0755)
	os.WriteFile(path.Join(projectRoot, "content", "first.md"), []byte(EmbedPost), 0755)
	os.WriteFile(path.Join(projectRoot, "templates", "base.html"), []byte(EmbedTemplateBase), 0755)
	os.WriteFile(path.Join(projectRoot, "templates", "index.html"), []byte(EmbedTemplateIndex), 0755)
	os.WriteFile(path.Join(projectRoot, "templates", "post.html"), []byte(EmbedTemplatePost), 0755)
	os.WriteFile(path.Join(projectRoot, "templates", "index.xml"), []byte(EmbedTemplateFeed), 0755)
}

func buildProject(projectRoot string) {
	// Read config file.
	configFilepath := path.Join(projectRoot, "config.yaml")
	configFile, err := os.ReadFile(configFilepath)
	if err != nil {
		panic(err)
	}
	config := Config{}
	err = yaml.Unmarshal(configFile, &config)
	if err != nil {
		panic(err)
	}

	// Gets the list of all markdown files.
	var files []string
	err = filepath.Walk(path.Join(projectRoot, "content/"), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && strings.ToLower(filepath.Ext(path)) == ".md" {
			files = append(files, path)
		}

		return nil
	})

	if err != nil {
		fmt.Printf("No markdown files found with error `%s`.\n", err)
		os.Exit(1)
	}

	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.Table,
			extension.TaskList,
			extension.Footnote,
			meta.Meta,
			highlighting.NewHighlighting(
				highlighting.WithStyle(config.Highlighting),
			),
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
			parser.WithBlockParsers(),
			parser.WithInlineParsers(),
			parser.WithParagraphTransformers(),
			parser.WithAttribute(),
		),
		goldmark.WithRendererOptions(
			html.WithXHTML(),
			html.WithUnsafe(),
		),
	)

	// Parse all markdown files in content folder.
	pages := []Page{}
	for _, file := range files {
		source, err := os.ReadFile(file)
		if err != nil {
			panic(err)
		}

		var buf bytes.Buffer
		ctx := parser.NewContext()
		if err := md.Convert(source, &buf, parser.WithContext(ctx)); err != nil {
			panic(err)
		}

		// Rank and summarize.
		tr := textrank.NewTextRank()
		rule := textrank.NewDefaultRule()
		language := textrank.NewDefaultLanguage()
		algorithmDef := textrank.NewDefaultAlgorithm()
		tr.Populate(cleanHTMLTags(buf.String()), language, rule)
		tr.Ranking(algorithmDef)

		sentences := textrank.FindSentencesByRelationWeight(tr, 50)
		sentences = textrank.FindSentencesFrom(tr, 0, 1)

		summary := ""
		for _, s := range sentences {
			summary = strings.ReplaceAll(s.Value, "\n", "")
		}

		metaData := meta.Get(ctx)
		t, _ := time.Parse("2006-01-02T15:04:05-07:00", metaData["date"].(string))
		pages = append(pages, Page{
			Filepath:     file,
			Meta:         metaData,
			Raw:          buf.String(),
			HTML:         template.HTML(buf.String()),
			Text:         cleanHTMLTags(buf.String()),
			Summary:      summary,
			Title:        metaData["title"].(string),
			Type:         metaData["type"].(string),
			RelPermalink: metaData["url"].(string),
			Created:      t,
			Draft:        metaData["draft"].(bool),
		})
	}

	// Sorting pages in descending created order.
	sort.Slice(pages, func(i, j int) bool {
		return pages[i].Created.After(pages[j].Created)
	})

	// Creates public folder if it doesn't exist yet.
	if err := os.Mkdir(path.Join(projectRoot, "public"), 0755); err != nil && !os.IsExist(err) {
		log.Println("Error creating directory:", err)
		return
	}

	// Generate HTML files for all pages.
	for _, page := range pages {
		outFilepath := path.Join(projectRoot, "public", page.Meta["url"].(string))
		if !page.Draft {
			pageTemplateFilename := fmt.Sprintf("%s.html", page.Meta["type"].(string))
			templatePathname := path.Join(projectRoot, "templates", pageTemplateFilename)
			baseTemplatePathname := path.Join(projectRoot, "templates/base.html")
			t, err := template.ParseFiles(baseTemplatePathname, templatePathname)
			if err != nil {
				panic(err)
			}

			type Payload struct {
				Config Config
				Page   Page
			}

			var buf bytes.Buffer
			err = t.Execute(&buf, Payload{
				Config: config,
				Page:   page,
			})
			if err != nil {
				panic(err)
			}

			outHTML := buf.String()
			if config.Minify {
				m := minify.New()
				m.AddFunc("text/html", mhtml.Minify)
				m.AddFunc("text/css", mcss.Minify)
				m.AddFunc("application/js", mjs.Minify)
				outHTML, err = m.String("text/html", outHTML)
				if err != nil {
					panic(err)
				}
			}

			os.WriteFile(outFilepath, []byte(outHTML), 0755)
			log.Println("Wrote", outFilepath)
		} else {
			log.Println("Skipped", outFilepath)
		}
	}

	// Generates index page.
	{
		log.Println("Writing index...")
		templatePathname := path.Join(projectRoot, "templates/index.html")
		baseTemplatePathname := path.Join(projectRoot, "templates/base.html")
		t, err := template.ParseFiles(baseTemplatePathname, templatePathname)
		if err != nil {
			panic(err)
		}

		type Payload struct {
			Config Config
			Pages  []Page
		}

		var buf bytes.Buffer
		err = t.Execute(&buf, Payload{
			Config: config,
			Pages:  pages,
		})
		if err != nil {
			panic(err)
		}

		outHTML := buf.String()
		if config.Minify {
			m := minify.New()
			m.AddFunc("text/html", mhtml.Minify)
			m.AddFunc("text/css", mcss.Minify)
			m.AddFunc("application/js", mjs.Minify)
			outHTML, err = m.String("text/html", outHTML)
			if err != nil {
				panic(err)
			}
		}

		outFilepath := path.Join(projectRoot, "public", "index.html")
		os.WriteFile(outFilepath, []byte(outHTML), 0755)
	}

	// Copy static files.
	{
		log.Println("Copying static files...")
		err := cp.Copy(path.Join(projectRoot, "static"), path.Join(projectRoot, "public"))
		if err != nil {
			panic(err)
		}
	}

	// Generates extras.
	{
		for _, extra := range config.Extras {
			log.Printf("Writing extras %s\n", extra.URL)
			templatePathname := path.Join(projectRoot, "templates", extra.Template)
			t, err := template.ParseFiles(templatePathname)
			if err != nil {
				panic(err)
			}

			type Payload struct {
				Config Config
				Pages  []Page
			}

			var buf bytes.Buffer
			err = t.Execute(&buf, Payload{
				Config: config,
				Pages:  pages,
			})
			if err != nil {
				panic(err)
			}

			outFilepath := path.Join(projectRoot, "public", extra.URL)
			os.WriteFile(outFilepath, []byte(buf.String()), 0755)
		}
	}

	// Guess we are done!
	log.Println("Done & done...")
}

func newPage(projectRoot string, title string) {
	slug := slug.Make(title)
	t := time.Now()
	filename := fmt.Sprintf("%s-%s.md", t.Format("2006-01-02"), slug)

	var lines = []string{
		"---",
		fmt.Sprintf("title: \"%s\"", title),
		fmt.Sprintf("url: %s.html", slug),
		fmt.Sprintf("date: %s", t.Format("2006-01-02T15:04:05-07:00")),
		"type: post",
		"draft: true",
		"---",
		"",
		"Content...",
	}

	f, err := os.Create(path.Join(projectRoot, "content", filename))
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	for _, line := range lines {
		_, err := f.WriteString(line + "\n")
		if err != nil {
			log.Fatal(err)
		}
	}

	log.Printf("Page `%s` created\n", filename)
}

func main() {
	projectRoot := os.Getenv("PROJECT_ROOT")
	if projectRoot == "" {
		projectRoot = "./"
	}

	var args struct {
		Init   bool   `arg:"-i,--init" help:"initialize new project"`
		Build  bool   `arg:"-b,--build" help:"build the website"`
		Server bool   `arg:"-s,--server" help:"simple embedded HTTP server"`
		New    bool   `arg:"-n,--new" help:"create new page"`
		Title  string `arg:"positional"`
	}

	arg.MustParse(&args)

	if !args.Init && !args.Build && !args.Server && !args.New {
		fmt.Println("No arguments provided. Try using `jbmafp --help`")
		os.Exit(0)
	}

	if args.Init {
		initializeProject(projectRoot)
	}

	if args.Build {
		buildProject(projectRoot)
	}

	if args.Server {
		simpleServer(projectRoot)
	}

	if args.New {
		if len(args.Title) == 0 {
			fmt.Println("You must provide a title for the new page")
			os.Exit(1)
		}
		newPage(projectRoot, args.Title)
	}
}
