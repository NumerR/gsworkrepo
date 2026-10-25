// Package security держит проверки паролей, email и токенов отдельно от HTTP
// и SQL: их можно перечитать и переиспользовать и в обработчиках, и в тестах.
package security

import (
	"crypto/sha256"
	"encoding/base64"
	"net/mail"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

const (
	// bcryptCost >= 10 требует Password Storage Cheat Sheet; 12 — это около
	// 0.3 с на регистрацию/вход, что и нужно: проверка должна быть медленной
	// для легального пользователя и неподъёмной для подбора.
	bcryptCost = 12

	minPasswordLen = 8   // V2.1: нижняя граница без требований к составу
	maxPasswordLen = 256 // верхняя граница только от DoS, а не из «правильного пароля»
	maxEmailLen    = 320 // разумный предел длины адреса из RFC 5321
)

// HashPassword считает bcrypt от SHA-256 пароля. Прехэш нужен потому, что
// bcrypt обрезает вход на 72 байтах: с ним работают и длинные passphrase,
// и полные Unicode-пароли, и одинаковая длина не выдаёт себя временем.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(preHash(password)), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// VerifyPassword сверяет пароль с хэшем. Ошибка здесь не различает «нет такого
// хэша» и «пароль неверный» — вызывающий код обязан отдать одно сообщение.
func VerifyPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(preHash(password))) == nil
}

// DummyHash — валидный хэш случайного пароля. Его сверяют, когда email не найден,
// чтобы ответ «нет такого аккаунта» не возвращался заметно быстрее ответа
// «неверный пароль» (WSTG-ATHN-04: перечисление пользователей по времени).
var DummyHash = mustHash("timing-equalizer-random-value")

func preHash(password string) string {
	sum := sha256.Sum256([]byte(password))
	return base64.StdEncoding.EncodeToString(sum[:])
}

func mustHash(password string) string {
	// Ошибка невозможна при фиксированном входе и валидной стоимости.
	hash, _ := HashPassword(password)
	return hash
}

// CheckPassword — отказ без подсказок о составе («добавьте цифру» подсказывает
// злоумышленнику структуру). Проверяется только длина и известность пароля.
func CheckPassword(password string) string {
	switch {
	case password == "":
		return "Введите пароль"
	case utf8.RuneCountInString(password) < minPasswordLen:
		return "Пароль должен быть не короче 8 символов"
	case utf8.RuneCountInString(password) > maxPasswordLen:
		return "Пароль слишком длинный"
	case strings.TrimSpace(password) != password:
		return "Пароль не должен начинаться или заканчиваться пробелом"
	case isCommon(password):
		return "Пароль слишком простой — возьмите другой"
	}
	return ""
}

// CheckEmail нормализует и проверяет форму адреса. Deliverability не проверяем:
// письмо подтверждением сброса пароля всё равно пока не отправляется.
func CheckEmail(email string) string {
	switch {
	case email == "":
		return "Введите email"
	case len(email) > maxEmailLen:
		return "Email слишком длинный"
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return "Email указан неверно"
	}
	return ""
}

func NormalizeEmail(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

func isCommon(password string) bool {
	_, ok := commonPasswords[strings.ToLower(password)]
	return ok
}

// CheckPasswordAgainstEmail запрещает пароль, совпадающий с логином (целиком
// или до «@») — такие пароли подбираются первым же правилом из словаря.
func CheckPasswordAgainstEmail(password, email string) string {
	local := email
	if at := strings.IndexByte(email, '@'); at >= 0 {
		local = email[:at]
	}
	if password == email || (local != "" && strings.EqualFold(password, local)) {
		return "Пароль не должен совпадать с email"
	}
	return ""
}

// commonPasswords — короткий стоп-список из публичных выборок утечек (top-100
// по разным сборникам, ru и en). V2.1.7 требует сверку со списком скомпрометированных
// паролей; здесь это локальный список, а не запрос к внешнему API (HIBP range
// требует сети и аккаунта).
var commonPasswords = map[string]struct{}{}

func init() {
	list := []string{
		"password", "password1", "password123", "p@ssw0rd", "passwd",
		"123456", "12345678", "123456789", "1234567890", "qwerty", "qwerty123",
		"qwertyuiop", "asdfghjkl", "abc123", "iloveyou", "letmein", "welcome",
		"admin", "admin123", "root", "toor", "login", "monkey", "dragon",
		"master", "football", "baseball", "superman", "batman", "trustno1",
		"sunshine", "princess", "welcome1", "hello", "mypass", "secret",
		"secret123", "changeme", "default", "test123", "guest", "111111",
		"000000", "123123", "654321", "696969", "zaq12wsx", "1q2w3e4r",
		"1qaz2wsx", "qazwsx", "zxcvbnm", "killer", "jordan", "hunter2",
		"buster", "soccer", "hockey", "rocky", "andrew", "daniel", "michael",
		"jennifer", "matrix", "starwars", "nintendo", "pokemon", "gundam",
		// русскоязычные выборки
		"пароль", "пароль123", "привет", "любовь", "собака", "котеньок",
		"королева", "пользователь", "админ", "супермен", "машина", "здравствуй",
		"1234567890abcdef", "qwertyuiopasdfgh", "password1234", "letmein123",
	}

	for _, password := range list {
		commonPasswords[password] = struct{}{}
	}
}
