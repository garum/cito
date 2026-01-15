package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
	"golang.org/x/oauth2"
)

type App struct {
	oauthConfig *oauth2.Config
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func (app *App) handler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	defer conn.Close()
	for {
		// read messsage
		_, p, err := conn.ReadMessage()
		if err != nil {
			log.Println(err)
			return
		}
		// echo in console
		log.Println(string(p[:]))
	}
}
func (app *App) loginHandler(w http.ResponseWriter, r *http.Request) {

	url := app.oauthConfig.AuthCodeURL("state")
	log.Printf("Visit the URL for the auth dialog: %v", url)
	html := fmt.Sprintf(`<a href="%s">Sign in with GitHub</a>`, url)
	w.Write([]byte(html))

	// tok, err := app.oauthConfig.Exchange(context.TODO(), "authorization-code")
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// client := app.oauthConfig.Client(context.TODO(), tok)
	// log.Println(client)
	//client.Get()

}

func (app *App) homeHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("homepage")
	w.Write([]byte("home!"))
}

func (app *App) callBackHandler(w http.ResponseWriter, r *http.Request) {
	// w.Write([]byte("Login mister!"))
	code := r.URL.Query().Get("code")
	log.Println("code:", code)
	tok, err := app.oauthConfig.Exchange(context.TODO(), code)
	if err != nil {
		log.Fatal(err)
		w.WriteHeader(http.StatusInternalServerError)
		return

	}
	log.Println(tok.AccessToken)
	log.Println(tok.RefreshToken)
	log.Println(tok.Expiry)
	log.Println(tok.TokenType)

	client := app.oauthConfig.Client(context.TODO(), tok)
	log.Println(client.Get("https://api.github.com/user"))
	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}
