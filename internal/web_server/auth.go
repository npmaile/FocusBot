package server

import (
	"crypto/rand"
	_ "embed"
	"fmt"
	"net/http"

	"github.com/gorilla/pat"
	"github.com/gorilla/sessions"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/discord"
	"github.com/npmaile/focusbot/pkg/logerooni"
)

func init() {
	var secret = make([]byte, 32)
	rand.Read(secret)
	gothic.Store = sessions.NewCookieStore(secret)
}

func setupAuth(key string, secret string, callbackURL string, scopes []string) *pat.Router {
	goth.UseProviders(discord.New(key, secret, callbackURL, scopes...))
	p := pat.New()
	p.Get("/auth/{provider}/callback", func(w http.ResponseWriter, r *http.Request) {
		user, err := gothic.CompleteUserAuth(w, r)
		if err != nil {
			fmt.Fprintln(w, err)
			return
		}
		logerooni.Infof("user %s has logged in", user.UserID)
		http.Redirect(w,r,"/management",http.StatusTemporaryRedirect)
	})

	p.Get("/auth/logout/{provider}", func(w http.ResponseWriter, r *http.Request) {
		user, _ := gothic.CompleteUserAuth(w, r)
		logerooni.Infof("user %s has logged out", user.UserID)
		gothic.Logout(w, r)
		w.Header().Set("Location", "/")
		w.WriteHeader(http.StatusTemporaryRedirect)
	})

	p.Get("/auth/{provider}", func(w http.ResponseWriter, r *http.Request) {
		// try to get the user without re-authenticating
		if _, err := gothic.CompleteUserAuth(w, r); err == nil {
			http.Redirect(w, r, "/index.html", http.StatusTemporaryRedirect)
		} else {
			gothic.BeginAuthHandler(w, r)
		}
	})
	return p
}

const loggedInTemplate = `
<p><a href="/auth/logout/{{.Provider}}">logout</a></p>
<p>Name: {{.Name}} [{{.LastName}}, {{.FirstName}}]</p>
<p>Email: {{.Email}}</p>
<p>NickName: {{.NickName}}</p>
<p>Location: {{.Location}}</p>
<p>AvatarURL: {{.AvatarURL}} <img src="{{.AvatarURL}}"></p>
<p>Description: {{.Description}}</p>
<p>UserID: {{.UserID}}</p>
<p>ExpiresAt: {{.ExpiresAt}}</p>
`
