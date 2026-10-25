package repository

import (
	"database/sql"
	"errors"
	"time"

	"gdwork/internal/security"
)

// ResetRepository — одноразовые токены сброса пароля. Как и с сессиями, в базе
// лежит только хэш: украденная таблица не даёт готовые ссылки для смены пароля.
type ResetRepository struct {
	db *sql.DB
}

func NewResetRepository(db *sql.DB) *ResetRepository {
	return &ResetRepository{db: db}
}

// Create выдаёт токен и гасит предыдущие токены этого пользователя: новый
// запрос сброса не должен оставлять живыми старые ссылки. Строки при этом не
// удаляются, а помечаются использованными — по ним считается лимит заявок за
// час (CountRecent), и удаление сделало бы лимит недостижимым.
func (r *ResetRepository) Create(userID int64, ttl time.Duration) (string, error) {
	token, err := security.NewToken()
	if err != nil {
		return "", err
	}

	tx, err := r.db.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	now := time.Now().Unix()
	if _, err := tx.Exec(`UPDATE password_resets SET used_at = ? WHERE user_id = ? AND used_at IS NULL`,
		now, userID); err != nil {
		return "", err
	}
	if _, err := tx.Exec(
		`INSERT INTO password_resets (user_id, token_hash, created_at, expires_at) VALUES (?, ?, ?, ?)`,
		userID, security.HashToken(token), now, now+int64(ttl.Seconds()),
	); err != nil {
		return "", err
	}

	return token, tx.Commit()
}

// Consume гасит токен и возвращает его пользователя. Один UPDATE с условиями
// «не использован и не просрочен» делает проверку и гашение атомарными:
// две параллельные попытки по одному токену не пройдут обе.
func (r *ResetRepository) Consume(token string) (int64, error) {
	now := time.Now().Unix()
	res, err := r.db.Exec(
		`UPDATE password_resets SET used_at = ?
			WHERE token_hash = ? AND used_at IS NULL AND expires_at > ?`,
		now, security.HashToken(token), now,
	)
	if err != nil {
		return 0, err
	}

	updated, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	if updated == 0 {
		return 0, ErrNotFound
	}

	var userID int64
	err = r.db.QueryRow(`SELECT user_id FROM password_resets WHERE token_hash = ?`,
		security.HashToken(token)).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	return userID, err
}

// CountRecent — сколько токенов пользователь запросил за окно: лимит на
// перебор по числу запросов сброса, а не только по числу попыток ввода.
func (r *ResetRepository) CountRecent(userID int64, window time.Duration) (int, error) {
	var count int
	err := r.db.QueryRow(
		`SELECT COUNT(*) FROM password_resets WHERE user_id = ? AND created_at > ?`,
		userID, time.Now().Add(-window).Unix(),
	).Scan(&count)
	return count, err
}

func (r *ResetRepository) DeleteExpired() error {
	_, err := r.db.Exec(`DELETE FROM password_resets WHERE expires_at <= ?`, time.Now().Unix())
	return err
}
