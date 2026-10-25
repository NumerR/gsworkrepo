package repository

import (
	"database/sql"

	"gdwork/internal/models"
)

// CategoryRepository читает справочник категорий.
type CategoryRepository struct {
	db *sql.DB
}

func NewCategoryRepository(db *sql.DB) *CategoryRepository {
	return &CategoryRepository{db: db}
}

// List — все категории в порядке задания.
func (r *CategoryRepository) List() ([]models.Category, error) {
	rows, err := r.db.Query(`SELECT id, name FROM categories ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []models.Category
	for rows.Next() {
		var c models.Category
		if err := rows.Scan(&c.ID, &c.Name); err != nil {
			return nil, err
		}
		categories = append(categories, c)
	}

	return categories, rows.Err()
}

// Exists — есть ли такая категория в справочнике; нужен для проверки фильтра.
func (r *CategoryRepository) Exists(id int64) (bool, error) {
	var one int
	err := r.db.QueryRow(`SELECT 1 FROM categories WHERE id = ?`, id).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}
