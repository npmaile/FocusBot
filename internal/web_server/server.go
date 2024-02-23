package server

import (
	"embed"
	"html/template"
	"io/fs"

	//	"mime"
	"net/http"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/markbates/goth/gothic"
	"github.com/npmaile/focusbot/internal/db"
	"github.com/npmaile/focusbot/internal/models"
	"github.com/npmaile/focusbot/pkg/logerooni"
)

//go:embed templates/content.template.html
var indexContent string
var indexTemplate *template.Template

//go:embed templates/management.template.html
var managementContent string
var managementTemplate *template.Template

//go:embed static/*
var static embed.FS

func init() {
	// pre-compile web templates
	var err error
	indexTemplate, err = template.New("index").Parse(indexContent)
	if err != nil {
		logerooni.Errorf("unable to parse indexTemplate: %s", err.Error())
	}
	managementTemplate, err = template.New("Mangement interface").Parse(managementContent)
	if err != nil {
		logerooni.Errorf("unable to parse managementTemplate: %s", err.Error())
	}

}

func SetupWebServer(clientID string, oauth2clientSecret string, dg *discordgo.Session, db db.DataStore) {
	realStatic, err := fs.Sub(static, "static")
	if err != nil {
		panic(err.Error())
	}
	var redirectURL = "http://localhost/auth/discord/callback"
	http.HandleFunc("/index.html", index(clientID, redirectURL))
	http.Handle("/auth/", setupAuth(clientID, oauth2clientSecret, redirectURL, []string{}))
	http.HandleFunc("/management/", managementPage)
	http.HandleFunc("/serverOptions/", serverOptionsFunc(dg, db))
	http.Handle("/", killFileIndex(http.FileServer(http.FS(realStatic))))
}

// //////////////////////////////////
// routes
// //////////////////////////////////
func index(clientID string, RedirectURL string) func(w http.ResponseWriter, _ *http.Request) {
	return func(w http.ResponseWriter, _ *http.Request) {
		var indexPageStuff = struct {
			ClientID    string
			RedirectURL string
		}{
			ClientID:    clientID,
			RedirectURL: RedirectURL,
		}
		err := indexTemplate.Execute(w, &indexPageStuff)
		if err != nil {
			logerooni.Errorf("unable to execute index template %s", err.Error())
		}
	}
}

func managementPage(w http.ResponseWriter, r *http.Request) {
	user, err := gothic.CompleteUserAuth(w, r)
	if err != nil {
		//todo something
		return
	}
	err = managementTemplate.Execute(w, user)
	if err != nil {
		logerooni.Errorf("problems %+v", err)
	}
}

func serversUserCanScrewWith(dg *discordgo.Session, db db.DataStore, UserID string) []*models.GuildConfig {
	db.GetServerConfiguration	

}

func serverOptionsFunc(dg *discordgo.Session, db db.DataStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, err := gothic.CompleteUserAuth(w, r)
		if err != nil {
			w.Write([]byte(`
		<p>
		something has gone terribly wrong. Please <a class="button" href="/">start over</a>
		</p>
		`))
			return
		}
		servers := serversUserCanScrewWith(user.UserID)

	}
}

const serverOptions = `
	
	{{ . }}
`

// ////////////////////////////////////
// middleware
// ///////////////////////////////////
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
