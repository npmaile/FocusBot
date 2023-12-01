package server

import (
	"fmt"
	"net/http"
)

//todo: replace with flashy interface!
func ServeLinkPageFunc(clientID string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, `
	<head>
		<title>
			"Please click the link to add the focus bot to your server"
		</title>
	</head>
	<body>
		<a href=https://discord.com/oauth2/authorize?client_id=%s&scope=bot&permissions=1> add to server!</a>
	</body`, clientID)
	}
}
