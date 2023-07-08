package main

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
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
	"github.com/microcosm-cc/bluemonday"

	"github.com/tdewolff/minify/v2"
	mcss "github.com/tdewolff/minify/v2/css"
	mhtml "github.com/tdewolff/minify/v2/html"
	mjs "github.com/tdewolff/minify/v2/js"

	highlighting "github.com/yuin/goldmark-highlighting/v2"

	cp "github.com/otiai10/copy"
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

// Function to clean HTML tags using bluemonday.
func cleanHTMLTags(htmlString string) string {
	p := bluemonday.StrictPolicy()
	cleanString := p.Sanitize(htmlString)
	return cleanString
}

func initializeProject(projectRoot string) {
	fmt.Println("Initializing new project")
}

func main() {
	projectRoot := os.Getenv("PROJECT_ROOT")
	if projectRoot == "" {
		projectRoot = "./"
	}

	fmt.Println("Come back later! Still WIP!")
	os.Exit(0)

	// Parsing arguments.
	var args struct {
		Init  bool `arg:"-i,--init" help:"initialize new project"`
		Build bool `arg:"-b,--build" help:"build the website"`
	}

	arg.MustParse(&args)

	if !args.Init && !args.Build {
		fmt.Println("No arguments provided. Try using `jbmafp --help`")
		os.Exit(0)
	}

	if args.Init {
		initializeProject(projectRoot)
		os.Exit(0)
	}

	os.Exit(0)

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
	files, err := filepath.Glob(path.Join(projectRoot, "content/*.md"))
	if err != nil {
		panic(err)
	}

	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
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
