package server

import (
	"embed"
	"html/template"
	"io/fs"
	//	"mime"
	"net/http"
	"strings"

	"github.com/npmaile/focusbot/pkg/logerooni"
)

//go:embed templates/content.template.html
var indexContent string
var indexTemplate *template.Template

//go:embed static/*
var static embed.FS

func init() {
	// pre-compile web templates
	var err error
	indexTemplate, err = template.New("index").Parse(indexContent)
	if err != nil {
		logerooni.Errorf("unable to parse indexTemplate: %s", err.Error())
	}
}

func SetupWebServer(clientID string, oauth2clientSecret string) {
	realStatic, err := fs.Sub(static, "static")
	if err != nil {
		panic(err.Error())
	}
	var redirectURL = "http://localhost/auth/discord/callback"
	http.HandleFunc("/index.html", index(clientID,redirectURL))
	http.Handle("/auth/", setupAuth(clientID, oauth2clientSecret, redirectURL, []string{}))
	http.Handle("/", killFileIndex(http.FileServer(http.FS(realStatic))))
}

func index(clientID string, RedirectURL string) func(w http.ResponseWriter, _ *http.Request) {
	return func(w http.ResponseWriter, _ *http.Request) {
		var indexPageStuff = struct{
			ClientID string	
			RedirectURL string
		}{
			ClientID: clientID,
			RedirectURL: RedirectURL,
		}
		err := indexTemplate.Execute(w, &indexPageStuff)
		if err != nil {
			logerooni.Errorf("unable to execute index template %s", err.Error())
		}
	}
}

func killFileIndex(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/index.html", http.StatusFound)
		}
		if strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}
