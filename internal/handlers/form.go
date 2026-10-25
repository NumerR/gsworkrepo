package handlers

import (
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gdwork/internal/models"
)

// expenseForm — данные для общего шаблона формы "expense-form".
type expenseForm struct {
	Action     string
	Submit     string
	Cancel     string
	Expense    models.Expense
	Categories []models.Category
	CSRF       string
}

func newExpenseForm(categories []models.Category, csrf string) expenseForm {
	return expenseForm{
		Action:     "/expenses",
		Submit:     "Добавить",
		Expense:    models.Expense{Date: time.Now().Format(time.DateOnly)},
		Categories: categories,
		CSRF:       csrf,
	}
}

// parseExpenseForm читает поля Description, Amount, Date, Category из формы.
func parseExpenseForm(r *http.Request) (models.Expense, error) {
	if err := r.ParseForm(); err != nil {
		return models.Expense{}, err
	}

	amount, err := parseAmount(r.FormValue("Amount"))
	if err != nil {
		return models.Expense{}, err
	}

	date := r.FormValue("Date")
	if _, err := time.Parse(time.DateOnly, date); err != nil {
		return models.Expense{}, err
	}

	// пусто — трата без категории; ноль приводит значение поля к CategoryID = 0
	category, err := strconv.ParseInt(r.FormValue("Category"), 10, 64)
	if err != nil && r.FormValue("Category") != "" {
		return models.Expense{}, err
	}

	return models.Expense{
		Amount:      amount,
		Description: strings.TrimSpace(r.FormValue("Description")),
		Date:        date,
		CategoryID:  category,
	}, nil
}

// parseAmount переводит "1234.5" или "1234,5" в копейки.
func parseAmount(s string) (int64, error) {
	s = strings.Replace(strings.TrimSpace(s), ",", ".", 1)

	f, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) || f <= 0 || f > 1e12 {
		return 0, errors.New("invalid amount")
	}

	return int64(math.Round(f * 100)), nil
}

func parseID(r *http.Request) (int64, error) {
	return strconv.ParseInt(r.PathValue("id"), 10, 64)
}
