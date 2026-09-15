package main

import (
	"fmt"
	"html/template"
	"net/http"

	"github.com/NumerR/gdwork/internal/handlers"
)

func main() {
	templates := template.Must(
		template.ParseGlob("templates/**/*.html"),
	)

	mux := http.NewServeMux()

	//Pages
	mux.HandleFunc("/", handlers.Home(templates))
	http.HandleFunc("/about", aboutHandler)
	http.HandleFunc("/ping", pingHandler)
	mux.HandleFunc("/login", handlers.Login())
	mux.HandleFunc("/register", handlers.Register())

	fmt.Println("Server started on http://localhost:8080")

	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		fmt.Println("Server error:", err)
	}
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "Welcome to Go server.")
}

func aboutHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "This is a simple HTTP server")
}

func pingHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	fmt.Fprintln(w, "pong")
}
