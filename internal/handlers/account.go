package handlers

import (
	"errors"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"time"

	"gdwork/internal/repository"
	"gdwork/internal/security"
)

const (
	// resetTTL — 15 минут по Cheat Sheet Forgot Password: ссылка живёт ровно
	// столько, сколько человек успевает её открыть из письма.
	resetTTL        = 15 * time.Minute
	resetUserWindow = time.Hour
	resetUserLimit  = 3
)

// Mailer — доставка ссылки сброса. Почтового транспорта в учебном проекте нет,
// поэтому ссылка уходит в серверный лог. Это DEV-режим: лог содержит живой
// токен, а production-transport (SMTP) подключается одной строкой в main.go.
type Mailer interface {
	SendResetLink(to, link string)
}

type logMailer struct{}

func (logMailer) SendResetLink(to, link string) {
	log.Println("DEV ONLY — ссылка сброса пароля для", to, "->", link)
}

func NewMailer() Mailer {
	return logMailer{}
}

// Account показывает аккаунт и форму смены пароля.
func Account(templates *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		noStore(w)

		render(w, r, templates, map[string]any{
			"Title":   "Аккаунт",
			"Info":    r.URL.Query().Get("changed") == "1",
			"Expired": r.URL.Query().Get("expired") == "1",
		})
	}
}

// ChangePassword меняет пароль по форме «текущий + новый».
func ChangePassword(
	templates *template.Template,
	users *repository.UserRepository,
	auth *Auth,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		noStore(w)

		user := currentUser(r)
		current := r.FormValue("password_current")
		next := r.FormValue("password")
		repeat := r.FormValue("password_repeat")

		// Смена пароля — та же проверка credentials, что и вход, поэтому
		// перебор текущего пароля закрыт тем же лимитером (V2.2.1).
		key := "chpwd:user:" + strconv.FormatInt(user.ID, 10)
		if wait := auth.limiter.Allowed(key); wait > 0 {
			tooManyAttempts(w, r, templates, "Аккаунт", wait)
			return
		}

		// Текущий пароль обязателен (V2.1.8): украденная сессия не должна
		// молча перебирать чужой аккаунт в одно нажатие.
		_, hash, err := users.ByEmail(user.Email)
		if err != nil && !errors.Is(err, repository.ErrNotFound) {
			serverError(w, err)
			return
		}
		if !security.VerifyPassword(hash, current) {
			log.Printf("auth: неудачная смена пароля user=%d", user.ID)
			auth.limiter.Fail(key, accountLockThreshold)
			render(w, r, templates, map[string]any{
				"Title": "Аккаунт",
				"Error": "Текущий пароль указан неверно",
			})
			return
		}

		if reason := security.CheckPassword(next); reason != "" {
			render(w, r, templates, map[string]any{
				"Title": "Аккаунт",
				"Error": reason,
			})
			return
		}
		if reason := security.CheckPasswordAgainstEmail(next, user.Email); reason != "" {
			render(w, r, templates, map[string]any{
				"Title": "Аккаунт",
				"Error": reason,
			})
			return
		}
		if next == current {
			render(w, r, templates, map[string]any{
				"Title": "Аккаунт",
				"Error": "Новый пароль совпадает с текущим",
			})
			return
		}
		if next != repeat {
			render(w, r, templates, map[string]any{
				"Title": "Аккаунт",
				"Error": "Новые пароли не совпадают",
			})
			return
		}

		newHash, err := security.HashPassword(next)
		if err != nil {
			serverError(w, err)
			return
		}
		if err := users.SetPassword(user.ID, newHash); err != nil {
			serverError(w, err)
			return
		}
		auth.limiter.Success(key)

		// V3.3: смена credentials обрывает все прежние сессии — эта пересоздаётся
		// следом, чтобы пользователя не выбросило на вход посреди работы.
		if err := auth.sessions.DeleteAll(user.ID); err != nil {
			serverError(w, err)
			return
		}
		if err := auth.Start(w, r, user.ID); err != nil {
			serverError(w, err)
			return
		}

		log.Printf("auth: пароль изменён user=%d", user.ID)
		http.Redirect(w, r, "/account?changed=1", http.StatusSeeOther)
	}
}

// ForgotPassword запрашивает ссылку сброса. Ответ не зависит от того, есть ли
// такой аккаунт, — иначе форма восстановления работает списком email (V2.3).
func ForgotPassword(
	templates *template.Template,
	users *repository.UserRepository,
	resets *repository.ResetRepository,
	mailer Mailer,
	auth *Auth,
) http.HandlerFunc {
	const title = "Восстановление пароля"

	return func(w http.ResponseWriter, r *http.Request) {
		noStore(w)

		if r.Method != http.MethodPost {
			render(w, r, templates, map[string]any{"Title": title})
			return
		}

		ipKey := "reset:ip:" + clientIP(r)
		if wait := auth.limiter.Allowed(ipKey); wait > 0 {
			tooManyAttempts(w, r, templates, title, wait)
			return
		}

		email, _ := authForm(r)
		const generic = "Если такой аккаунт существует, мы уже отправили ссылку для сброса."

		user, _, err := users.ByEmail(email)
		if errors.Is(err, repository.ErrNotFound) {
			// Тот же bcrypt-«расход», что и при входе: ответ на неизвестный
			// адрес не должен приходить заметно быстрее ответа на известный.
			security.VerifyPassword(security.DummyHash, "timing-equalizer")
			render(w, r, templates, map[string]any{"Title": title, "Info": generic})
			return
		}
		if err != nil {
			serverError(w, err)
			return
		}

		recent, err := resets.CountRecent(user.ID, resetUserWindow)
		if err != nil {
			serverError(w, err)
			return
		}
		if recent >= resetUserLimit {
			// Превышен лимит на пользователя — ответ тот же, что и обычно:
			// количество заявок не должно угадываться по странице.
			log.Printf("auth: лимит заявок на сброс user=%d", user.ID)
			render(w, r, templates, map[string]any{"Title": title, "Info": generic})
			return
		}

		token, err := resets.Create(user.ID, resetTTL)
		if err != nil {
			serverError(w, err)
			return
		}

		mailer.SendResetLink(email, "/reset?token="+token)
		auth.limiter.Success(ipKey)
		log.Printf("auth: запрошен сброс пароля user=%d", user.ID)
		render(w, r, templates, map[string]any{"Title": title, "Info": generic})
	}
}

// ResetPassword принимает токен и новый пароль.
func ResetPassword(
	templates *template.Template,
	users *repository.UserRepository,
	resets *repository.ResetRepository,
	auth *Auth,
) http.HandlerFunc {
	const title = "Новый пароль"

	return func(w http.ResponseWriter, r *http.Request) {
		noStore(w)

		token := r.FormValue("token")

		if r.Method != http.MethodPost {
			// Форма показывается и по битой ссылке: «токен неверен» только
			// после отправки, чтобы GET-проверка не подбирала токены.
			render(w, r, templates, map[string]any{"Title": title, "Token": token})
			return
		}

		ipKey := "reset:ip:" + clientIP(r)
		if wait := auth.limiter.Allowed(ipKey); wait > 0 {
			tooManyAttempts(w, r, templates, title, wait)
			return
		}

		password := r.FormValue("password")
		if reason := security.CheckPassword(password); reason != "" {
			// Проверка до гашения токена: слабый пароль не должен сжигать
			// единственную выданную ссылку.
			render(w, r, templates, map[string]any{
				"Title": title, "Token": token, "Error": reason,
			})
			return
		}
		if password != r.FormValue("password_repeat") {
			render(w, r, templates, map[string]any{
				"Title": title, "Token": token, "Error": "Пароли не совпадают",
			})
			return
		}

		userID, err := resets.Consume(token)
		if errors.Is(err, repository.ErrNotFound) {
			auth.limiter.Fail(ipKey, ipLockThreshold)
			render(w, r, templates, map[string]any{
				"Title": title, "Token": token, "Error": "Ссылка недействительна или уже использована",
			})
			return
		}
		if err != nil {
			serverError(w, err)
			return
		}

		hash, err := security.HashPassword(password)
		if err != nil {
			serverError(w, err)
			return
		}
		if err := users.SetPassword(userID, hash); err != nil {
			serverError(w, err)
			return
		}

		// Сброс через почту сильнее всего похож на угон аккаунта, поэтому
		// рвём все сессии и не входим автоматически (V3.3, V2.1).
		if err := auth.sessions.DeleteAll(userID); err != nil {
			serverError(w, err)
			return
		}

		auth.limiter.Success(ipKey)
		log.Printf("auth: пароль сброшен user=%d", userID)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	}
}
