package main

import (
	"html/template"
	"log"
	"net/http"

	"gdwork/internal/handlers"
)

func main() {

	templates := template.Must(
		template.ParseGlob("web/template/**/*.html"),
	)

	mux := http.NewServeMux()

	// Pages
	mux.HandleFunc("/", handlers.Home(templates))
	mux.HandleFunc("/about", handlers.About)
	mux.HandleFunc("/ping", handlers.Ping)
	mux.HandleFunc("/login", handlers.Login(templates))
	mux.HandleFunc("/register", handlers.Register(templates))

	// Static files

	fileServer := http.FileServer(http.Dir("./web/static"))

	mux.Handle(
		"/static/",
		http.StripPrefix("/static/", fileServer),
	)

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	log.Println("Server started on http://localhost:8080")

	err := server.ListenAndServe()
	if err != nil {
		log.Fatal(err)
	}
}
