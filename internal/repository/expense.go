package repository

import (
	"database/sql"
	"errors"
	"strings"

	"gdwork/internal/models"
)

var ErrNotFound = errors.New("expense not found")

// ExpenseFilter — необязательные условия выборки списка трат.
// Нулевые значения означают «фильтр не задан».
type ExpenseFilter struct {
	CategoryID int64
	From       string // YYYY-MM-DD, включительно
	To         string // YYYY-MM-DD, включительно
}

// ExpenseRepository содержит весь SQL для работы с тратами.
// Значения передаются только через placeholder ?, а не склейкой строк.
type ExpenseRepository struct {
	db *sql.DB
}

func NewExpenseRepository(db *sql.DB) *ExpenseRepository {
	return &ExpenseRepository{db: db}
}

// Название категории берётся тем же запросом через LEFT JOIN: на список — один
// запрос, а не запрос на каждую строку (проблема N+1).
const expenseSelect = `SELECT e.id, e.amount, e.description, e.date,
        COALESCE(e.category_id, 0), COALESCE(c.name, '')
    FROM expenses e
    LEFT JOIN categories c ON c.id = e.category_id`

func (r *ExpenseRepository) Create(e *models.Expense) error {
	res, err := r.db.Exec(
		`INSERT INTO expenses (amount, description, date, category_id) VALUES (?, ?, ?, ?)`,
		e.Amount, e.Description, e.Date, categoryKey(e.CategoryID),
	)
	if err != nil {
		return err
	}

	e.ID, err = res.LastInsertId()
	return err
}

// List — траты, подходящие под фильтр; порядок — от свежих к старым.
func (r *ExpenseRepository) List(f ExpenseFilter) ([]models.Expense, error) {
	query := strings.Builder{}
	query.WriteString(expenseSelect)

	var conditions []string
	var args []any

	if f.CategoryID > 0 {
		conditions = append(conditions, "e.category_id = ?")
		args = append(args, f.CategoryID)
	}
	if f.From != "" {
		conditions = append(conditions, "e.date >= ?")
		args = append(args, f.From)
	}
	if f.To != "" {
		conditions = append(conditions, "e.date <= ?")
		args = append(args, f.To)
	}

	if len(conditions) > 0 {
		query.WriteString(" WHERE " + strings.Join(conditions, " AND "))
	}
	query.WriteString(" ORDER BY e.date DESC, e.id DESC")

	rows, err := r.db.Query(query.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var expenses []models.Expense
	for rows.Next() {
		var e models.Expense
		if err := rows.Scan(&e.ID, &e.Amount, &e.Description, &e.Date,
			&e.CategoryID, &e.CategoryName); err != nil {
			return nil, err
		}
		expenses = append(expenses, e)
	}

	return expenses, rows.Err()
}

func (r *ExpenseRepository) GetByID(id int64) (models.Expense, error) {
	var e models.Expense

	err := r.db.QueryRow(expenseSelect+" WHERE e.id = ?", id).Scan(
		&e.ID, &e.Amount, &e.Description, &e.Date, &e.CategoryID, &e.CategoryName,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return e, ErrNotFound
	}
	return e, err
}

func (r *ExpenseRepository) Update(e models.Expense) error {
	res, err := r.db.Exec(
		`UPDATE expenses SET amount = ?, description = ?, date = ?, category_id = ? WHERE id = ?`,
		e.Amount, e.Description, e.Date, categoryKey(e.CategoryID), e.ID,
	)
	if err != nil {
		return err
	}
	return checkAffected(res)
}

func (r *ExpenseRepository) Delete(id int64) error {
	res, err := r.db.Exec(`DELETE FROM expenses WHERE id = ?`, id)
	if err != nil {
		return err
	}
	return checkAffected(res)
}

func checkAffected(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// categoryKey хранит «без категории» как NULL, а не как 0 — иначе внешний ключ не сойдётся.
func categoryKey(id int64) any {
	if id == 0 {
		return nil
	}
	return id
}
