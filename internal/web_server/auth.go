package server

import (
	"crypto/rand"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/gorilla/pat"
	"github.com/gorilla/sessions"
	"github.com/markbates/goth"
	"github.com/markbates/goth/providers/discord"
	"github.com/npmaile/focusbot/internal/db"
	"github.com/npmaile/focusbot/internal/gothic"
	"github.com/npmaile/focusbot/internal/models"
	"github.com/npmaile/focusbot/pkg/logerooni"
)

func init() {
	var secret = make([]byte, 32)
	rand.Read(secret)
	gothic.Store = sessions.NewCookieStore(secret)
}

func setupAuth(key string, secret string, callbackURL string, scopes []string, dbInstance db.DataStore) *pat.Router {
	goth.UseProviders(discord.New(key, secret, callbackURL, scopes...))
	p := pat.New()
	p.Get("/auth/{provider}/callback", func(w http.ResponseWriter, r *http.Request) {
		user, err := gothic.CompleteUserAuth(w, r)
		if err != nil {
			fmt.Fprintln(w, err)
			return
		}
		//here
		guildInfoPath := "/users/@me/guilds"
		discordAPIBase := "https://discord.com/api"
		req, err := http.NewRequest(http.MethodGet, discordAPIBase+guildInfoPath, nil)
		if err != nil {
			panic(err.Error())
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Add("Authorization", "Bearer "+user.AccessToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			panic(err.Error())
		}
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			panic(err.Error())
		}

		servers := []Server{}
		err = json.Unmarshal(b, &servers)
		if err != nil {
			panic(err.Error())
		}

		var ids []string
		for _, serv := range servers {
			ids = append(ids, serv.ID)
		}
		guilds2return, err := dbInstance.GetStupidDBIntersect(ids)
		if err != nil {
			panic(err.Error())
		}

		type megaGuildConfig struct {
			*models.GuildConfig
			ServerName string
		}

		var ret []megaGuildConfig
		for _, guild := range guilds2return {
			for _, server := range servers {
				if guild.ID == server.ID && server.Permissions&discordgo.PermissionManageServer != 0 {
					ret = append(ret, megaGuildConfig{
						guild,
						server.Name,
					})
				}
			}
		}
		json.NewEncoder(w).Encode(ret)
		// shove this shit into a token

		// redirect

	})

	p.Get("/auth/logout/{provider}", func(w http.ResponseWriter, r *http.Request) {
		user, _ := gothic.CompleteUserAuth(w, r)
		logerooni.Infof("user %s has logged out", user.UserID)
		gothic.Logout(w, r)
		w.Header().Set("Location", "/")
		w.WriteHeader(http.StatusTemporaryRedirect)
	})

	p.Get("/auth/{provider}/testShit", func(w http.ResponseWriter, r *http.Request) {
		// try to get the user without re-authenticating
		user, err := gothic.CompleteUserAuth(w, r)
		if err != nil {
			panic(err.Error())
		}
		fmt.Println("1")
		guildInfoPath := "/users/@me/guilds"
		discordAPIBase := "https://discord.com/api"
		req, err := http.NewRequest(http.MethodGet, discordAPIBase+guildInfoPath, nil)
		if err != nil {
			panic(err.Error())
		}
		fmt.Println("2")
		req.Header.Set("Accept", "application/json")
		req.Header.Add("authorization", "bearer "+user.AccessToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			panic(err.Error())
		}
		fmt.Println("3")
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			panic(err.Error())
		}

		servers := []Server{}
		err = json.Unmarshal(b, &servers)
		if err != nil {
			panic(err.Error())
		}

		//		fmt.Printf("%+v", servers)

		var ids []string
		for _, serv := range servers {
			ids = append(ids, serv.ID)
		}
		fmt.Println(strings.Join(ids, ","))
		fmt.Println("48")
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

type Server struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Icon           string `json:"icon"`
	PermissionsNew string `json:"permissions_new"`
	Permissions    int64  `json:"permissions"`
	Owner          bool   `json:"owner"`
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
