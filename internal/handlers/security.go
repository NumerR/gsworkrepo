package handlers

import (
	"context"
	"net/http"
	"strings"

	"gdwork/internal/security"
)

const (
	csrfCookieName = "csrf"
	csrfFieldName  = "csrf"

	// maxBodyBytes — формы приложения маленькие; мегабайта хватает с запасом,
	// а без лимита один POST с телом на гигабайт съедает память и трафик.
	maxBodyBytes = 1 << 20
)

// csrfContextKey — токен, который шаблоны кладут в скрытое поле формы.
type csrfContextKey struct{}

// Security — заголовки безопасности, CSRF и ограничение размера тела запроса.
type Security struct {
	signer *security.Signer
}

func NewSecurity(signer *security.Signer) *Security {
	return &Security{signer: signer}
}

// Wrap оборачивает весь маршрутизатор: заголовки нужны и статике (nosniff,
// frame-ancestors), а CSRF проверяется до любой обработчик-логики.
func (s *Security) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secure := r.TLS != nil
		setSecurityHeaders(w, secure)

		if r.ContentLength > maxBodyBytes {
			http.Error(w, "Слишком длинный запрос", http.StatusRequestEntityTooLarge)
			return
		}

		if isStateChanging(r.Method) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
			if !s.checkCSRF(w, r) {
				return
			}
		}

		token := s.ensureCSRF(w, r, secure)
		next.ServeHTTP(w, r.WithContext(withCSRF(r.Context(), token)))
	})
}

// ensureCSRF выдаёт пару «cookie + поле формы». Значения различаются: в cookie
// лежит случайная часть, в форму — HMAC от неё. Поэтому подстановка cookie
// с поддомена (cookie tossing) токен не подделывает, а форма без cookie вообще
// не проходит проверку.
func (s *Security) ensureCSRF(w http.ResponseWriter, r *http.Request, secure bool) string {
	cookie, err := r.Cookie(csrfCookieName)
	if err != nil || cookie.Value == "" {
		value, err := security.NewToken()
		if err != nil {
			return ""
		}
		http.SetCookie(w, &http.Cookie{
			Name:     csrfCookieName,
			Value:    value,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Secure:   secure,
		})
		return s.signer.Sign(value)
	}

	return s.signer.Sign(cookie.Value)
}

// checkCSRF сравнивает поле формы с подписью из cookie. Одинаковый ответ для
// «поля нет», «cookie нет» и «не совпало» — 403 без подсказок.
func (s *Security) checkCSRF(w http.ResponseWriter, r *http.Request) bool {
	cookie, err := r.Cookie(csrfCookieName)
	if err != nil || cookie.Value == "" {
		http.Error(w, "Форма устарела — обновите страницу", http.StatusForbidden)
		return false
	}

	// ParseForm у POST читает тело один раз, обработчики получат те же значения.
	supplied := r.FormValue(csrfFieldName)
	if supplied == "" || !s.signer.Verify(cookie.Value, supplied) {
		http.Error(w, "Форма устарела — обновите страницу", http.StatusForbidden)
		return false
	}
	return true
}

func isStateChanging(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	}
	return false
}

func setSecurityHeaders(w http.ResponseWriter, secure bool) {
	h := w.Header()
	h.Set("Content-Security-Policy", cspPolicy)
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "same-origin")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Cross-Origin-Opener-Policy", "same-origin")
	h.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=(), payment=(), usb=()")

	if secure {
		// HSTS имеет смысл только там, где есть HTTPS: на http его игнорируют,
		// а отправить его с http-страницы — значит обещать то, чего нет.
		h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
	}
}

// cspPolicy собран под то, что реально грузит страница: свои скрипты и стили,
// Google Fonts, data: для шеврона select из CSS. 'unsafe-inline' отсутствует
// специально — инлайновый скрипт темы вынесен в /static/js/theme.js.
// При подключении Chart.js (этап 6) CDN нужно будет добавить в script-src.
const cspPolicy = "default-src 'self'; " +
	"script-src 'self'; " +
	"style-src 'self' https://fonts.googleapis.com; " +
	"font-src 'self' https://fonts.gstatic.com; " +
	"img-src 'self' data:; " +
	"connect-src 'self'; " +
	"form-action 'self'; " +
	"base-uri 'self'; " +
	"object-src 'none'; " +
	"frame-src 'none'; " +
	"frame-ancestors 'none'"

// noStore запрещает кэшировать приватные ответы: список трат, страницы удаления
// и аккаунт не должны остаться в кэше браузера или общей машины (WSTG-ATHN-03).
func noStore(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Cache-Control", "no-store, max-age=0")
	h.Set("Pragma", "no-cache")
}

// safeNext разрешает возврат только на свои пути: «//evil.com», «http://evil.com»
// и «/\evil.com» — это вектор open redirect (WSTG-CLNT-04), поэтому оставляем
// относительный путь из allowlist. Слэш внутри пути легален («/expenses?to=»),
// поэтому сравнение идёт по отдельным признакам, а не по набору символов.
func safeNext(raw string) string {
	if raw == "" || !strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "//") {
		return ""
	}
	if strings.ContainsAny(raw, `\@`) || strings.Contains(raw, "://") ||
		strings.ContainsRune(raw, '\n') || strings.ContainsRune(raw, '\r') {
		return ""
	}
	switch {
	case raw == "/":
		return raw
	case strings.HasPrefix(raw, "/expenses"), strings.HasPrefix(raw, "/account"):
		return raw
	}
	return ""
}

func withCSRF(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, csrfContextKey{}, token)
}

// csrfToken — подпись для скрытого поля формы текущей страницы.
func csrfToken(r *http.Request) string {
	token, _ := r.Context().Value(csrfContextKey{}).(string)
	return token
}
