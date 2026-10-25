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

// Login показывает форму входа и проверяет пароль.
func Login(
	templates *template.Template,
	users *repository.UserRepository,
	auth *Auth,
) http.HandlerFunc {
	const title = "Вход"

	return func(w http.ResponseWriter, r *http.Request) {
		noStore(w)

		if r.Method != http.MethodPost {
			renderAuth(w, r, templates, title, "", http.StatusOK, "")
			return
		}

		ipKey := "login:ip:" + clientIP(r)
		if wait := auth.limiter.Allowed(ipKey); wait > 0 {
			tooManyAttempts(w, r, templates, title, wait)
			return
		}

		email, password := authForm(r)
		if email == "" || password == "" {
			renderAuth(w, r, templates, title, email, http.StatusOK, "Заполните email и пароль")
			return
		}

		emailKey := "login:email:" + email
		if wait := auth.limiter.Allowed(emailKey); wait > 0 {
			tooManyAttempts(w, r, templates, title, wait)
			return
		}

		user, hash, err := users.ByEmail(email)
		switch {
		case errors.Is(err, repository.ErrNotFound):
			// Сверяется хэш заведомо чужого пароля, чтобы ответ «нет такого
			// адреса» не возвращался быстрее ответа «неверный пароль» —
			// иначе страница входа выдаёт список зарегистрированных email
			// по времени ответа (WSTG-ATHN-04).
			hash = security.DummyHash
		case err != nil:
			serverError(w, err)
			return
		}

		if !security.VerifyPassword(hash, password) {
			blocked := auth.limiter.Fail(ipKey, ipLockThreshold) || auth.limiter.Fail(emailKey, accountLockThreshold)
			log.Printf("auth: неудачный вход email=%s ip=%s blocked=%v", email, clientIP(r), blocked)
			renderAuth(w, r, templates, title, email, http.StatusUnauthorized, "Неверный email или пароль")
			return
		}

		auth.limiter.Success(ipKey)
		auth.limiter.Success(emailKey)

		if err := auth.Start(w, r, user.ID); err != nil {
			serverError(w, err)
			return
		}

		log.Printf("auth: вход выполнен user=%d", user.ID)
		http.Redirect(w, r, afterLoginURL(r), http.StatusSeeOther)
	}
}

const (
	// registerQuotaLimit — сколько аккаунтов можно завести с одного адреса за окно.
	// Значение заведомо щедрее, чем нужно человеку: адрес часто общий (NAT
	// провайдера или кампусная сеть), и жёсткий лимит наказал бы живых людей.
	// Смысл квоты — остановить автоматическую накрутку аккаунтов, а не мешать людям.
	registerQuotaLimit  = 30
	registerQuotaWindow = failWindow
)

// Register создаёт аккаунт с bcrypt-хэшем пароля и сразу входит в него.
func Register(
	templates *template.Template,
	users *repository.UserRepository,
	auth *Auth,
) http.HandlerFunc {
	const title = "Регистрация"

	return func(w http.ResponseWriter, r *http.Request) {
		noStore(w)

		if r.Method != http.MethodPost {
			renderAuth(w, r, templates, title, "", http.StatusOK, "")
			return
		}

		ipKey := "register:ip:" + clientIP(r)
		if wait := auth.limiter.Allowed(ipKey); wait > 0 {
			tooManyAttempts(w, r, templates, title, wait)
			return
		}
		if !auth.limiter.Quota("register:quota:ip:"+clientIP(r), registerQuotaLimit, registerQuotaWindow) {
			log.Printf("auth: превышена квота регистраций ip=%s", clientIP(r))
			renderAuth(w, r, templates, title, "", http.StatusTooManyRequests,
				"Слишком много регистраций с этого адреса. Попробуйте позже.")
			return
		}

		email, password := authForm(r)
		if reason := checkRegistration(email, password, r.FormValue("password_repeat")); reason != "" {
			renderAuth(w, r, templates, title, email, http.StatusOK, reason)
			return
		}

		// Открытый пароль в базу не уходит: bcrypt считает хэш со случайной
		// солью, и обратное преобразование невозможно.
		hash, err := security.HashPassword(password)
		if err != nil {
			serverError(w, err)
			return
		}

		user, err := users.Create(email, hash)
		if repository.EmailTaken(err) {
			// Занятый email — не признак подбора, поэтому в лимитер не идёт:
			// иначе пользователь с общей сетью потерял бы регистрацию целиком.
			// Массовую регистрацию останавливает лимит по IP выше.
			renderAuth(w, r, templates, title, email, http.StatusOK, "Такой email уже зарегистрирован")
			return
		}
		if err != nil {
			serverError(w, err)
			return
		}

		auth.limiter.Success(ipKey)

		if err := auth.Start(w, r, user.ID); err != nil {
			serverError(w, err)
			return
		}

		log.Printf("auth: зарегистрирован user=%d", user.ID)
		http.Redirect(w, r, "/expenses", http.StatusSeeOther)
	}
}

// authForm берёт email и пароль из формы. Email нормализуем: условие UNIQUE
// чувствительно к регистру, а «A@b.ru» и «a@b.ru» — один и тот же ящик.
func authForm(r *http.Request) (email, password string) {
	return security.NormalizeEmail(r.FormValue("email")), r.FormValue("password")
}

// checkRegistration — минимум, нужный уже сейчас: длина, пробелы по краям,
// стоп-список утечек и совпадение повтора. Правил «добавьте цифру» сознательно
// нет: они подсказывают злоумышленнику структуру пароля (V2.1).
func checkRegistration(email, password, repeat string) string {
	for _, reason := range []string{
		security.CheckEmail(email),
		security.CheckPassword(password),
		security.CheckPasswordAgainstEmail(password, email),
	} {
		if reason != "" {
			return reason
		}
	}
	if password != repeat {
		return "Пароли не совпадают"
	}
	return ""
}

// requestedNext — разрешённый адрес возврата из запроса: скрытое поле формы
// или query ссылки, по которой её открыли. У POST-запроса FormValue отдаёт
// значение тела, поэтому пустое поле не должно затирать параметр query, а
// чужой адрес («//evil.com») отбрасывает safeNext.
func requestedNext(r *http.Request) string {
	for _, raw := range []string{r.FormValue("next"), r.URL.Query().Get("next")} {
		if next := safeNext(raw); next != "" {
			return next
		}
	}
	return ""
}

// afterLoginURL решает, куда вести после входа: свой путь из next либо список трат.
func afterLoginURL(r *http.Request) string {
	if next := requestedNext(r); next != "" {
		return next
	}
	return "/expenses"
}

func tooManyAttempts(
	w http.ResponseWriter,
	r *http.Request,
	templates *template.Template,
	title string,
	wait time.Duration,
) {
	minutes := int(wait.Round(time.Minute).Minutes())
	if minutes < 1 {
		minutes = 1
	}

	log.Printf("auth: сработал лимит попыток ip=%s", clientIP(r))
	renderAuth(w, r, templates, title, "", http.StatusTooManyRequests,
		"Слишком много попыток. Подождите "+strconv.Itoa(minutes)+" мин и попробуйте снова.")
}

// renderAuth отдаёт страницу входа/регистрации. Код 401 и 429 важны для тестов
// и для клиента, но текст остаётся общим: «неверный email» и «неверный пароль»
// неразличимы, зарегистрирован ли адрес.
func renderAuth(
	w http.ResponseWriter,
	r *http.Request,
	templates *template.Template,
	title, email string,
	status int,
	reason string,
) {
	data := map[string]any{
		"Title": title,
		"Email": email,
		// в форму кладётся уже проверенный next: поле не должно становиться
		// хранилищем произвольного адреса, даже если сервер перепроверяет его сам
		"Next": requestedNext(r),
	}
	if reason != "" {
		data["Error"] = reason
	}

	if status != http.StatusOK {
		w.WriteHeader(status)
	}

	render(w, r, templates, data)
}
