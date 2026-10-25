package repository

import (
	"database/sql"
	"errors"

	sqlite "modernc.org/sqlite"

	"gdwork/internal/models"
)

// UserRepository — доступ к таблице users.
type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

// EmailTaken — нарушение UNIQUE на users.email. Проверяем код ошибки драйвера,
// а не предварительный SELECT: проверка «занято ли» отдельно от вставки оставила бы
// гонку, когда две регистрации с одним почтовым ящиком проходят одновременно.
func EmailTaken(err error) bool {
	var dbErr *sqlite.Error
	return errors.As(err, &dbErr) && dbErr.Code() == sqliteConstraintUnique
}

const sqliteConstraintUnique = 2067 // SQLITE_CONSTRAINT_UNIQUE

func (r *UserRepository) Create(email, passwordHash string) (models.User, error) {
	res, err := r.db.Exec(`INSERT INTO users (email, password_hash) VALUES (?, ?)`, email, passwordHash)
	if err != nil {
		return models.User{}, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return models.User{}, err
	}

	return models.User{ID: id, Email: email}, nil
}

// ByEmail возвращает аккаунт и хэш его пароля — он нужен вызывающему коду
// только для сверки и не попадает в модель User.
func (r *UserRepository) ByEmail(email string) (models.User, string, error) {
	var (
		user models.User
		hash string
	)

	err := r.db.QueryRow(
		`SELECT id, email, password_hash FROM users WHERE email = ?`, email,
	).Scan(&user.ID, &user.Email, &hash)
	if errors.Is(err, sql.ErrNoRows) {
		return user, "", ErrNotFound
	}
	if err != nil {
		return user, "", err
	}

	return user, hash, nil
}

// SetPassword меняет хэш пароля. Вызывающий код обязан следом порвать все
// сессии пользователя и перевыпустить cookie (V3.3.2, V2.1.8).
func (r *UserRepository) SetPassword(userID int64, passwordHash string) error {
	res, err := r.db.Exec(`UPDATE users SET password_hash = ? WHERE id = ?`, passwordHash, userID)
	if err != nil {
		return err
	}
	return checkAffected(res)
}
