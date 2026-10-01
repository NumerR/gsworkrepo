package main

import (
	"html/template"
	"log"
	"net/http"

	"gdwork/internal/handlers"
	"gdwork/internal/models"
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
	expensesTemplate := template.Must(
		template.ParseFiles(
			"web/template/layout/base.html",
			"web/template/components/header.html",
			"web/template/pages/expense.html",
		))

	mux := http.NewServeMux()

	expenses := []models.Expense{}

	// Страницы
	mux.HandleFunc("/", handlers.Home(homeTemplate))
	mux.HandleFunc("/about", handlers.About)
	mux.HandleFunc("/ping", handlers.Ping)
	mux.HandleFunc("/login", handlers.Login(loginTemplate))
	mux.HandleFunc("/register", handlers.Register(registerTemplate))
	mux.HandleFunc("/expenses", handlers.ExpenseList(expensesTemplate, &expenses))
	mux.HandleFunc("POST /expenses", handlers.AddExpense(&expenses))

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
