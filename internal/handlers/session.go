package handlers

import (
	"context"
	"errors"
	"log"
	"net/http"
	"net/url"
	"time"

	"gdwork/internal/models"
	"gdwork/internal/repository"
)

const (
	sessionCookieName = "session"

	// sessionIdleTTL — срок простоя: закрытая вкладка на ночь разлогинивает.
	// V3.3 для «общего компьютера» просит 15–30 минут; для личного трекера
	// выбрана компромиссная граница, она меняется одной строкой.
	sessionIdleTTL     = 12 * time.Hour
	sessionAbsoluteTTL = 7 * 24 * time.Hour
	sessionRenewalStep = 30 * time.Minute
)

// userContextKey — свой тип вместо строки: ключ строкой мог бы совпасть
// с ключом другой библиотеки в том же контексте.
type userContextKey struct{}

// Auth — единственная точка, где приложение решает, кто авторизован.
type Auth struct {
	sessions *repository.SessionRepository
	limiter  *Limiter
}

func NewAuth(sessions *repository.SessionRepository, limiter *Limiter) *Auth {
	return &Auth{sessions: sessions, limiter: limiter}
}

// Required пускает дальше только с валидной сессией; гость уходит на страницу
// входа, а исходный путь возвращается через ?next= (валидируется в safeNext).
func (a *Auth) Required(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, err := a.resolve(w, r)
		if err != nil {
			serverError(w, err)
			return
		}
		if user.ID == 0 {
			http.Redirect(w, r, loginURL(r), http.StatusSeeOther)
			return
		}

		next(w, r.WithContext(withUser(r.Context(), user)))
	}
}

// Optional ведёт себя как Required, но гостя не блокирует: главная страница
// показывает разный контент в зависимости от авторизации.
func (a *Auth) Optional(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, err := a.resolve(w, r)
		if err != nil {
			serverError(w, err)
			return
		}

		next(w, r.WithContext(withUser(r.Context(), user)))
	}
}

// Start создаёт сессию пользователя и отдаёт токен в cookie. Прежняя сессия,
// если она была в cookie, удаляется: вход всегда выпускает новый токен, поэтому
// токен, подставленный до аутентификации (session fixation, V3.2.2), не переживает
// вход и остаётся висеть мёртвой строкой, а не живой сессией.
func (a *Auth) Start(w http.ResponseWriter, r *http.Request, userID int64) error {
	if old, err := r.Cookie(sessionCookieName); err == nil && old.Value != "" {
		if err := a.sessions.Delete(old.Value); err != nil {
			return err
		}
	}

	token, err := a.sessions.Create(userID, sessionIdleTTL)
	if err != nil {
		return err
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
		Expires:  time.Now().Add(sessionIdleTTL),
		MaxAge:   int(sessionIdleTTL.Seconds()),
		// Secure включается только вместе с HTTPS: на локальном http браузер
		// такую cookie не отдаст вовсе. __Host-session требует Secure и отказа
		// от Path — добавим тем же условием при переходе на HTTPS.
	})

	return nil
}

// Terminate снимает сессию: удаляет строку из базы и забирает cookie. Нужны оба
// действия — cookie можно восстановить из истории браузера, а запись в базе без
// удаления осталась бы живой.
func (a *Auth) Terminate(w http.ResponseWriter, r *http.Request) error {
	if cookie, err := r.Cookie(sessionCookieName); err == nil && cookie.Value != "" {
		if err := a.sessions.Delete(cookie.Value); err != nil {
			return err
		}
	}
	expireSessionCookie(w)
	return nil
}

// Logout — выход по POST (CSRF проверяет middleware): ссылка-картинка с чужого
// сайта не должна заканчиваться разлогиниванием.
func (a *Auth) Logout(w http.ResponseWriter, r *http.Request) {
	if err := a.Terminate(w, r); err != nil {
		serverError(w, err)
		return
	}

	log.Println("auth: logout")
	noStore(w)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// Cleanup удаляет сессии, у которых истёк хотя бы один из двух сроков.
// Метод на Auth, а не вызов репозитория из main.go: длительности знает только
// здесь, и они не должны разъехаться с проверкой в resolve.
func (a *Auth) Cleanup() error {
	return a.sessions.DeleteExpired(sessionAbsoluteTTL)
}

// resolve достаёт пользователя из cookie и продлевает срок простоя.
// Нулевой User — гость: cookie нет, она просрочена или сессия отозвана.
func (a *Auth) resolve(w http.ResponseWriter, r *http.Request) (models.User, error) {
	cookie, err := r.Cookie(sessionCookieName)
	if errors.Is(err, http.ErrNoCookie) || cookie.Value == "" {
		return models.User{}, nil
	}
	if err != nil {
		return models.User{}, err
	}

	user, err := a.sessions.User(cookie.Value, sessionAbsoluteTTL)
	if errors.Is(err, repository.ErrNotFound) {
		// Иначе браузер будет отправлять мёртвую cookie при каждом запросе.
		expireSessionCookie(w)
		return models.User{}, nil
	}
	if err != nil {
		return models.User{}, err
	}

	// Ошибка продления не рвёт сессию: запрос всё равно авторизован,
	// а дедлайн успеет обновиться при следующем.
	if err := a.sessions.Touch(cookie.Value, sessionIdleTTL, sessionAbsoluteTTL, sessionRenewalStep); err != nil {
		log.Println("session touch failed:", err)
	}

	return user, nil
}

// loginURL сохраняет исходный путь, чтобы после входа вернуться туда, а не
// на /expenses с потерянными фильтрами.
func loginURL(r *http.Request) string {
	target := r.URL.Path
	if r.URL.Path == "/expenses" {
		// фильтры — часть среза списка, их тоже возвращаем во вход
		if encoded := r.URL.RawQuery; encoded != "" {
			target += "?" + encoded
		}
	}

	if next := safeNext(target); next != "" {
		return "/login?next=" + url.QueryEscape(next)
	}
	return "/login"
}

func withUser(ctx context.Context, user models.User) context.Context {
	return context.WithValue(ctx, userContextKey{}, user)
}

// currentUser — пользователь из контекста, установленного middleware.
// Ноль означает гостя, поэтому шаблоны проверяют .User.ID, а не .User.
func currentUser(r *http.Request) models.User {
	user, _ := r.Context().Value(userContextKey{}).(models.User)
	return user
}

func expireSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}
