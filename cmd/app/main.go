package main

import (
	"html/template"
	"log"
	"net/http"

	"gdwork/internal/database"
	"gdwork/internal/handlers"
	"gdwork/internal/repository"
)

func main() {

	db, err := database.Open("expenses.db?_pragma=busy_timeout(5000)")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := database.Migrate(db, "migrations"); err != nil {
		log.Fatal(err)
	}

	expenseRepo := repository.NewExpenseRepository(db)

	homeTemplate := page("home")
	registerTemplate := page("register")
	loginTemplate := page("login")
	expensesTemplate := page("expense")
	editTemplate := page("expense_edit")
	deleteTemplate := page("expense_delete")

	mux := http.NewServeMux()

	// Страницы
	mux.HandleFunc("/", handlers.Home(homeTemplate))
	mux.HandleFunc("/about", handlers.About)
	mux.HandleFunc("/ping", handlers.Ping)
	mux.HandleFunc("/login", handlers.Login(loginTemplate))
	mux.HandleFunc("/register", handlers.Register(registerTemplate))

	// Траты
	mux.HandleFunc("GET /expenses", handlers.ExpenseList(expensesTemplate, expenseRepo))
	mux.HandleFunc("POST /expenses", handlers.AddExpense(expenseRepo))
	mux.HandleFunc("GET /expenses/{id}/edit", handlers.EditExpense(editTemplate, expenseRepo))
	mux.HandleFunc("POST /expenses/{id}/edit", handlers.UpdateExpense(expenseRepo))
	mux.HandleFunc("GET /expenses/{id}/delete", handlers.ConfirmDeleteExpense(deleteTemplate, expenseRepo))
	mux.HandleFunc("POST /expenses/{id}/delete", handlers.DeleteExpense(expenseRepo))

	// Static files

	fileServer := http.FileServer(http.Dir("./web/static"))

	// no-cache заставляет браузер проверять свежесть CSS/JS при каждой загрузке
	mux.Handle(
		"/static/",
		http.StripPrefix("/static/", noCache(fileServer)),
	)

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	log.Println("Server started on http://localhost:8080")

	err = server.ListenAndServe()
	if err != nil {
		log.Fatal(err)
	}
}

// page собирает шаблон страницы вместе с общим layout и компонентами
func page(name string) *template.Template {
	return template.Must(
		template.ParseFiles(
			"web/template/layout/base.html",
			"web/template/components/header.html",
			"web/template/components/expense_form.html",
			"web/template/pages/"+name+".html",
		))
}

func noCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		next.ServeHTTP(w, r)
	})
}
