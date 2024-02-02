package server

import (
	"bytes"
	"embed"
	"html/template"
	"io/fs"
	"mime"
	"net/http"
	"strings"

	"github.com/npmaile/focusbot/pkg/logerooni"
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
	// pre-compile web templates
	var err error
	boilerplateTemplate, err = template.New("boilerplate").Parse(boilerplate)
	if err != nil {
		logerooni.Errorf("unable to parse indexTemplate: %s", err.Error())
	}

	indexTemplate, err = template.New("index").Parse(indexContent)
	if err != nil {
		logerooni.Errorf("unable to parse indexTemplate: %s", err.Error())
	}

	// add some mime types
	mime.AddExtensionType(".js", "application/javascript")
	mime.AddExtensionType(".mp4", "video/mp4")

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
		err := indexTemplate.Execute(buf, template.HTML(clientID))
		if err != nil{
			logerooni.Errorf("unable to execute index template %s", err.Error())
		}
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
