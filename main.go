package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/Nikkolix/webserver"
	et "github.com/barasher/go-exiftool"
	"log"
	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

var src = flag.String("src", "invalid", "source directory")

func htmlBase(child gomponents.Node) gomponents.Node {
	return html.Doctype(html.HTML(
		html.Lang("en"),
		html.Head(
			html.Link(html.Href("style.css"), html.Rel("stylesheet")),
			html.Script(
				html.Src("https://unpkg.com/htmx.org@2.0.3"),
				html.Integrity("sha384-0895/pl2MU10Hqc6jd4RvrthNlDiE9U1tWmX7WRESftEDRosgxNsQG/Ze9YMRzHq"),
				html.CrossOrigin("anonymous"),
			),
		),
		html.Body(gomponents.Attr("hx-swap", "outerHTML"), child),
	))
}

func view(imgSrc string, tagged map[string]bool) gomponents.Node {
	var tags []gomponents.Node

	tags = append(tags, html.Class("flex-center"))

	for i, tagName := range availableTags {

		var checked gomponents.Node = nil
		if tagged[tagName] {
			checked = html.Checked()
		}

		tags = append(tags,
			html.Span(
				html.Input(
					html.Type("checkbox"),
					checked,
					gomponents.Attr("hx-post", "/tag"),
					gomponents.Attr("hx-target", "body"),
					gomponents.Attr("hx-trigger", "click"),
					gomponents.Attr("hx-vals", "{\"Name\":\""+tagName+"\"}"),
					html.AutoComplete("off"),
				),
				html.Label(
					gomponents.Text(tagName),
					html.Style("color: rgb("+colors[i%len(colors)]+");"),
				),
			),
		)
	}

	return html.Div(
		html.Div(
			html.Class("flex-center"),
			html.Img(
				html.Src(imgSrc),
				html.Class("img"),
			),
		),
		html.Div(
			html.Class("flex-center"),
			html.Button(
				gomponents.Attr("hx-post", "/next"),
				gomponents.Attr("hx-target", "body"),
				gomponents.Attr("hx-trigger", "click"),
				gomponents.Text("next"),
			),
		),
		html.Div(
			html.Class("flex-center"),
			html.Input(
				gomponents.Attr("hx-post", "/new-tag"),
				gomponents.Attr("hx-target", "body"),
				gomponents.Attr("hx-trigger", "change"),
				html.Type("text"),
				html.Name("Name"),
				html.Placeholder("new tag name"),
				html.AutoComplete("off"),
			),
		),
		html.Div(
			tags...,
		),
	)
}

func sendMainPage(rw http.ResponseWriter, req *http.Request) {
	tagged := make(map[string]bool)

	file := dir[currentFileIndex]

	name := file.Name()
	path := filepath.Join(*src, name)

	meta := tool.ExtractMetadata(path)

	for _, m := range meta {
		value, err := m.GetString("XPKeywords")
		taggedTags := strings.Split(value, ";")

		for _, tag := range taggedTags {
			if !slices.Contains(availableTags, tag) && tag != "" {
				availableTags = append(availableTags, tag)
			}
		}

		if err != nil {
			if errors.Is(err, et.ErrKeyNotFound) {
				continue
			}
			log.Fatal(err)
		}
		for _, tag := range availableTags {
			tagged[tag] = tagged[tag] || slices.Contains(taggedTags, tag)
		}
	}

	fmt.Println(meta)

	err := htmlBase(view(path, tagged)).Render(rw)
	if err != nil {
		log.Fatal(err)
	}
}

var currentFileIndex = 0
var availableTags = make([]string, 0)
var dir []os.DirEntry

var tool *et.Exiftool

var colors = []string{"230,25,75", "60,180,75", "255,225,25", "0,130,200", "245,130,48", "145,30,180", "70,240,240", "240,50,230", "210,245,60", "250,190,212", "0,128,128", "220,190,255", "170,110,40", "255,250,200", "128,0,0", "170,255,195", "128,128,0", "255,215,180", "0,0,128", "128,128,128", "255,255,255", "0,0,0"}

type NewTagData struct {
	Name string
}

type TagData struct {
	Name string
}

func main() {
	var err error

	tool, err = et.NewExiftool()
	if err != nil {
		log.Fatal(err)
	}
	defer func(tool *et.Exiftool) {
		err := tool.Close()
		if err != nil {
			log.Fatal(err)
		}
	}(tool)

	flag.Parse()

	if *src == "invalid" {
		log.Fatalln("source directory must be set")
	}

	dir, err = os.ReadDir(*src)
	if err != nil {
		log.Fatal(err)
	}

	settings := webserver.NewSettings()
	settings.FallbackRedirect = "/index"

	server := webserver.NewWebServer(*settings)

	server.NewHandleFunc(webserver.HTTPMethodGet, "/index", sendMainPage)

	server.NewHandleFunc(webserver.HTTPMethodPost, "/next", func(rw http.ResponseWriter, req *http.Request) {
		currentFileIndex++
		sendMainPage(rw, req)
	})

	webserver.NewURLBodyHandler(server, webserver.HTTPMethodPost, "/new-tag", func(rw http.ResponseWriter, req *http.Request, t NewTagData) {
		if t.Name != "" {
			availableTags = append(availableTags, t.Name)
		}
		sendMainPage(rw, req)
	})

	webserver.NewURLBodyHandler(server, webserver.HTTPMethodPost, "/tag", func(rw http.ResponseWriter, req *http.Request, t TagData) {

		file := dir[currentFileIndex]

		name := file.Name()
		path := filepath.Join(*src, name)

		meta := tool.ExtractMetadata(path)

		if len(meta) == 0 {
			meta = append(meta, et.EmptyFileMetadata())
		}

		tags, err := meta[0].GetString("XPKeywords")
		if err != nil {
			if !errors.Is(err, et.ErrKeyNotFound) {
				log.Fatal(err)
			}

		}

		contains := slices.Contains(strings.Split(tags, ";"), t.Name)

		if contains {
			parts := strings.Split(tags, ";")
			newParts := make([]string, 0)

			for _, tag := range parts {
				if tag != t.Name {
					newParts = append(newParts, tag)
				}
			}

			tags = strings.Join(newParts, ";")
		} else {
			if tags == "" {
				tags = t.Name
			} else {
				tags += ";" + t.Name
			}
		}

		meta[0].SetString("XPKeywords", tags)

		fmt.Println(meta)

		tool.WriteMetadata(meta)

		fmt.Println(meta)

		sendMainPage(rw, req)
	})

	err = server.Run()
	if err != nil {
		log.Fatal(err)
	}
}
