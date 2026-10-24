package repository

import (
	"database/sql"
	"errors"

	"gdwork/internal/models"
)

var ErrNotFound = errors.New("expense not found")

// ExpenseRepository содержит весь SQL для работы с тратами.
// Значения передаются только через placeholder ?, а не склейкой строк.
type ExpenseRepository struct {
	db *sql.DB
}

func NewExpenseRepository(db *sql.DB) *ExpenseRepository {
	return &ExpenseRepository{db: db}
}

func (r *ExpenseRepository) Create(e *models.Expense) error {
	res, err := r.db.Exec(
		`INSERT INTO expenses (amount, description, date) VALUES (?, ?, ?)`,
		e.Amount, e.Description, e.Date,
	)
	if err != nil {
		return err
	}

	e.ID, err = res.LastInsertId()
	return err
}

func (r *ExpenseRepository) GetAll() ([]models.Expense, error) {
	rows, err := r.db.Query(
		`SELECT id, amount, description, date FROM expenses ORDER BY date DESC, id DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var expenses []models.Expense
	for rows.Next() {
		var e models.Expense
		if err := rows.Scan(&e.ID, &e.Amount, &e.Description, &e.Date); err != nil {
			return nil, err
		}
		expenses = append(expenses, e)
	}

	return expenses, rows.Err()
}

func (r *ExpenseRepository) GetByID(id int64) (models.Expense, error) {
	var e models.Expense
	err := r.db.QueryRow(
		`SELECT id, amount, description, date FROM expenses WHERE id = ?`, id,
	).Scan(&e.ID, &e.Amount, &e.Description, &e.Date)

	if errors.Is(err, sql.ErrNoRows) {
		return e, ErrNotFound
	}
	return e, err
}

func (r *ExpenseRepository) Update(e models.Expense) error {
	res, err := r.db.Exec(
		`UPDATE expenses SET amount = ?, description = ?, date = ? WHERE id = ?`,
		e.Amount, e.Description, e.Date, e.ID,
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
