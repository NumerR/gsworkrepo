package main

import (
	"html/template"
	"log"
	"net/http"

	"gdwork/internal/handlers"
)

func main() {

	homeTemplate := template.Must(
		template.ParseFiles(
			"web/template/layout/base.html",
			"web/template/components/header.html",
			"web/template/pages/home.html",
		))
	registerTemplate := template.Must(
		template.ParseFiles(
			"web/template/layout/base.html",
			"web/template/components/header.html",
			"web/template/pages/register.html",
		))
	loginTemplate := template.Must(
		template.ParseFiles(
			"web/template/layout/base.html",
			"web/template/components/header.html",
			"web/template/pages/login.html",
		))
	mux := http.NewServeMux()

	// Pages
	mux.HandleFunc("/", handlers.Home(homeTemplate))
	mux.HandleFunc("/about", handlers.About)
	mux.HandleFunc("/ping", handlers.Ping)
	mux.HandleFunc("/login", handlers.Login(loginTemplate))
	mux.HandleFunc("/register", handlers.Register(registerTemplate))

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
