package main

import (
	"html/template"
	"log"
	"net/http"
	"time"

	"gdwork/internal/database"
	"gdwork/internal/handlers"
	"gdwork/internal/repository"
	"gdwork/internal/security"
)

const (
	// Таймауты сервера (V5.1, WSTG-CONF-04): без них один медленно читающий
	// клиент держит goroutine и сокет открытыми бесконечно — это slowloris.
	readHeaderTimeout = 5 * time.Second
	writeTimeout      = 20 * time.Second
	readTimeout       = 20 * time.Second
	idleTimeout       = 60 * time.Second
	maxHeaderBytes    = 1 << 16
)

func main() {

	// foreign_keys включает проверку внешних ключей: без неё SQLite принимает
	// category_id, которого нет в справочнике
	db, err := database.Open("expenses.db?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := database.Migrate(db, "migrations"); err != nil {
		log.Fatal(err)
	}

	expenseRepo := repository.NewExpenseRepository(db)
	categoryRepo := repository.NewCategoryRepository(db)
	userRepo := repository.NewUserRepository(db)
	sessionRepo := repository.NewSessionRepository(db)
	resetRepo := repository.NewResetRepository(db)

	signer, err := security.NewSigner()
	if err != nil {
		log.Fatal(err)
	}

	auth := handlers.NewAuth(sessionRepo, handlers.NewLimiter())
	securityMiddleware := handlers.NewSecurity(signer)

	// Чистка просроченных сессий и токенов сброса: проверка срока есть и в
	// запросе выбора, но без удаления таблицы росли бы бесконечно.
	if err := auth.Cleanup(); err != nil {
		log.Fatal(err)
	}
	if err := resetRepo.DeleteExpired(); err != nil {
		log.Fatal(err)
	}

	homeTemplate := page("home")
	registerTemplate := page("register")
	loginTemplate := page("login")
	accountTemplate := page("account")
	forgotTemplate := page("forgot")
	resetTemplate := page("reset")
	expensesTemplate := page("expense")
	editTemplate := page("expense_edit")
	deleteTemplate := page("expense_delete")

	login := handlers.Login(loginTemplate, userRepo, auth)
	register := handlers.Register(registerTemplate, userRepo, auth)
	forgot := handlers.ForgotPassword(forgotTemplate, userRepo, resetRepo, handlers.NewMailer(), auth)
	reset := handlers.ResetPassword(resetTemplate, userRepo, resetRepo, auth)

	mux := http.NewServeMux()

	// Страницы
	// Главная видит гостя и авторизованного, поэтому Optional
	mux.HandleFunc("/", auth.Optional(handlers.Home(homeTemplate, categoryRepo)))
	mux.HandleFunc("/about", handlers.About)
	mux.HandleFunc("/ping", handlers.Ping)
	mux.HandleFunc("GET /login", login)
	mux.HandleFunc("POST /login", login)
	mux.HandleFunc("GET /register", register)
	mux.HandleFunc("POST /register", register)
	mux.HandleFunc("POST /logout", auth.Logout)
	mux.HandleFunc("GET /forgot", forgot)
	mux.HandleFunc("POST /forgot", forgot)
	mux.HandleFunc("GET /reset", reset)
	mux.HandleFunc("POST /reset", reset)

	// Аккаунт — приватная страница, доступ через тот же middleware, что и траты
	mux.HandleFunc("GET /account", auth.Required(handlers.Account(accountTemplate)))
	mux.HandleFunc("POST /account/password", auth.Required(handlers.ChangePassword(accountTemplate, userRepo, auth)))

	// Траты — доступ только через middleware: по прямой ссылке гость
	// оказывается на /login, а не на чужом списке
	mux.HandleFunc("GET /expenses", auth.Required(handlers.ExpenseList(expensesTemplate, expenseRepo, categoryRepo)))
	mux.HandleFunc("POST /expenses", auth.Required(handlers.AddExpense(expenseRepo, categoryRepo)))
	mux.HandleFunc("GET /expenses/{id}/edit", auth.Required(handlers.EditExpense(editTemplate, expenseRepo, categoryRepo)))
	mux.HandleFunc("POST /expenses/{id}/edit", auth.Required(handlers.UpdateExpense(expenseRepo, categoryRepo)))
	mux.HandleFunc("GET /expenses/{id}/delete", auth.Required(handlers.ConfirmDeleteExpense(deleteTemplate, expenseRepo)))
	mux.HandleFunc("POST /expenses/{id}/delete", auth.Required(handlers.DeleteExpense(expenseRepo)))

	// Static files

	fileServer := http.FileServer(http.Dir("./web/static"))

	// no-cache заставляет браузер проверять свежесть CSS/JS при каждой загрузке
	mux.Handle(
		"/static/",
		http.StripPrefix("/static/", noCache(fileServer)),
	)

	server := &http.Server{
		Addr:              ":8080",
		Handler:           securityMiddleware.Wrap(mux),
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    maxHeaderBytes,
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
