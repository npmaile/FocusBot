package server

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed templates/content.template.html
var indexContent string
var indexTemplate *template.Template

//go:embed templates/boilerplate.template.html
var boilerplate string
var boilerplateTemplate *template.Template

//go:embed templates/static/*
var static embed.FS

func init() {
	var err error
	boilerplateTemplate, err = template.New("boilerplate").Parse(boilerplate)
	if err != nil {
		fmt.Println(err.Error())
	}

	indexTemplate, err = template.New("index").Parse(indexContent)
	if err != nil {
		fmt.Println(err.Error())
	}
}

func SetupWebServer(clientID string) {
	realStatic, err := fs.Sub(static, "static")
	if err != nil {
		panic(err.Error())
	}
	http.HandleFunc("/index.html", index(clientID))
	http.Handle("/", killFileIndex(http.FileServer(http.FS(realStatic))))
}

func index(clientID string) func(w http.ResponseWriter, _ *http.Request) {
	return func(w http.ResponseWriter, _ *http.Request) {
		buf := bytes.NewBuffer([]byte{})
		indexTemplate.Execute(buf, clientID)
		boilerplateTemplate.Execute(w, template.HTML(buf.String()))
	}
}

func killFileIndex(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}
