package main

import (
	"log"
	"net/http"

	"golang.org/x/oauth2"
)

func main() {

	conf := &oauth2.Config{
		ClientID:     "Ov23liaDFeoEST27gLF4",
		ClientSecret: "77ffe20c21ee800f0352defc8a7d19f5d3195d55",
		Scopes:       []string{"user"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://github.com/login/oauth/authorize",
			TokenURL: "https://github.com/login/oauth/access_token",
		},
		RedirectURL: "http://127.0.0.1:8080/oauth2/callback",
	}

	//conf.Client()
	mux := http.NewServeMux()

	log.Println("start web socket listen")

	app := &App{oauthConfig: conf}

	mux.HandleFunc("/ws", app.handler)
	mux.HandleFunc("/login", app.loginHandler)
	mux.HandleFunc("/oauth2/callback", app.callBackHandler)
	mux.HandleFunc("/", app.homeHandler)

	server := http.Server{Addr: "127.0.0.1:8080", Handler: mux}
	log.Println(server.Addr)
	log.Fatal(server.ListenAndServe())
	log.Println("close server")
}
