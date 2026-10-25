package security

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

// NewToken — 32 байта из crypto/rand (256 бит). V3.1 требует минимум 64 бит
// энтропии и генератор, который нельзя воспроизвести: math/rand и «email +
// timestamp» не подходят.
func NewToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// HashToken — в базе хранится SHA-256 токена, а сам токен уходит только в cookie.
// Дамп таблицы sessions не должен превращаться в набор действующих сессий.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// Signer подписывает значения для CSRF-пары cookie+поле формы. Секрет живёт
// в памяти процесса, поэтому после рестарта старые подписи перестают сходиться:
// форма отдаёт 403 и перезагружается со свежим токеном.
type Signer struct {
	secret []byte
}

func NewSigner() (*Signer, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return nil, err
	}
	return &Signer{secret: buf}, nil
}

// Sign — base64 от HMAC-SHA256. Значение в поле формы не равно содержимому
// cookie: атакующий, способный только подставить cookie (cookie tossing
// с поддомена), подпись не подделает.
func (s *Signer) Sign(value string) string {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(value))
	return base64.RawStdEncoding.EncodeToString(mac.Sum(nil))
}

// Verify — hmac.Equal сравнивает без утечки времени через длительность сравнения.
func (s *Signer) Verify(value, signature string) bool {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(value))
	got, err := base64.RawStdEncoding.DecodeString(signature)
	if err != nil {
		return false
	}
	return hmac.Equal(mac.Sum(nil), got)
}
