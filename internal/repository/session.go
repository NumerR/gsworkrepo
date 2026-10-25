package repository

import (
	"database/sql"
	"errors"
	"time"

	"gdwork/internal/models"
	"gdwork/internal/security"
)

// SessionRepository — сессии в таблице sessions, а не в карте памяти:
// карта живёт до перезапуска процесса и только в одном инстансе, а таблица
// переживает рестарт сервера. Плата — запрос к базе на каждый проверяемый запрос.
type SessionRepository struct {
	db *sql.DB
}

func NewSessionRepository(db *sql.DB) *SessionRepository {
	return &SessionRepository{db: db}
}

// Create отдаёт токен для cookie и запоминает только его хэш. expires_at —
// дедлайн простоя, created_at — начало абсолютного срока.
func (r *SessionRepository) Create(userID int64, idle time.Duration) (string, error) {
	token, err := security.NewToken()
	if err != nil {
		return "", err
	}

	now := time.Now().Unix()
	_, err = r.db.Exec(
		`INSERT INTO sessions (id, user_id, created_at, expires_at) VALUES (?, ?, ?, ?)`,
		security.HashToken(token), userID, now, now+int64(idle.Seconds()),
	)
	if err != nil {
		return "", err
	}

	return token, nil
}

// User находит владельца живой сессии. Проверяются оба срока: expires_at
// (простой) и created_at + absolute (общая жизнь сессии, которую скольжение
// не отодвигает). Без проверки сроков токен из cookie стал бы бессрочным,
// а «Выйти» и «сменить пароль» перестали что-либо значить.
func (r *SessionRepository) User(token string, absolute time.Duration) (models.User, error) {
	var user models.User

	now := time.Now().Unix()
	err := r.db.QueryRow(
		`SELECT u.id, u.email
			FROM sessions s
			JOIN users u ON u.id = s.user_id
			WHERE s.id = ? AND s.expires_at > ? AND s.created_at > ?`,
		security.HashToken(token), now, now-int64(absolute.Seconds()),
	).Scan(&user.ID, &user.Email)
	if errors.Is(err, sql.ErrNoRows) {
		return user, ErrNotFound
	}
	if err != nil {
		return user, err
	}

	return user, nil
}

// Touch продлевает срок простоя, но не пишет в базу на каждом запросе:
// строка обновляется, только когда до конца idle осталось меньше renewalStep.
func (r *SessionRepository) Touch(token string, idle, absolute, renewalStep time.Duration) error {
	now := time.Now().Unix()

	_, err := r.db.Exec(
		`UPDATE sessions SET expires_at = MIN(?, created_at + ?)
			WHERE id = ? AND expires_at < ?`,
		now+int64(idle.Seconds()), int64(absolute.Seconds()),
		security.HashToken(token), now+int64((idle-renewalStep).Seconds()),
	)
	return err
}

func (r *SessionRepository) Delete(token string) error {
	_, err := r.db.Exec(`DELETE FROM sessions WHERE id = ?`, security.HashToken(token))
	return err
}

// DeleteAll рвёт все сессии пользователя — обязательно после смены или сброса
// пароля (V3.3): украденная сессия не должна пережить смену credentials.
func (r *SessionRepository) DeleteAll(userID int64) error {
	_, err := r.db.Exec(`DELETE FROM sessions WHERE user_id = ?`, userID)
	return err
}

// DeleteExpired чистит базу от протухших записей, иначе таблица растёт бесконечно.
func (r *SessionRepository) DeleteExpired(absolute time.Duration) error {
	now := time.Now().Unix()
	_, err := r.db.Exec(`DELETE FROM sessions WHERE expires_at <= ? OR created_at <= ?`,
		now, now-int64(absolute.Seconds()))
	return err
}
