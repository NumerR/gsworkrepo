package handlers

import (
	"gdwork/internal/models"
	"html/template"
	"net/http"
	"strconv"
)

// принимает данные из формы
func AddExpense(expenses *[]models.Expense) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()

		description := r.FormValue("Description")
		amount := r.FormValue("Amount")
		date := r.FormValue("Date")

		amount, err := strconv.Atoi(amountStr)
		if err != nil {
			http.Error(w, "Неверная сумма", http.StatusBadRequest)
			return
		}

		models.Expense{
			Description: description,
			Amount:      amount,
			Date:        dateStr,
		}

		*expenses = append(*expenses, models.Expense{})
		http.Redirect(w, r, "/expenses", http.StatusSeeOther)
	}
}

// показывает сатрницу со списком трат
func ExpenseList(templates *template.Template, expenses *[]models.Expense) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		templates.ExecuteTemplate(w, "base", map[string]any{
			"Expenses": *expenses,
		})
	}

}
