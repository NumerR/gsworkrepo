package handlers

import (
	"errors"
	"html/template"
	"log"
	"net/http"
	"strconv"

	"gdwork/internal/models"
	"gdwork/internal/repository"
)

// ExpenseList показывает страницу со списком трат
func ExpenseList(templates *template.Template, repo *repository.ExpenseRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		expenses, err := repo.GetAll()
		if err != nil {
			serverError(w, err)
			return
		}

		render(w, templates, map[string]any{
			"Title":    "Траты",
			"Expenses": expenses,
			"Form":     newExpenseForm(),
		})
	}
}

// AddExpense принимает данные из формы и сохраняет трату
func AddExpense(repo *repository.ExpenseRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		expense, err := parseExpenseForm(r)
		if err != nil {
			http.Error(w, "Неверные данные формы", http.StatusBadRequest)
			return
		}

		if err := repo.Create(&expense); err != nil {
			serverError(w, err)
			return
		}

		http.Redirect(w, r, "/expenses?flash=add&id="+strconv.FormatInt(expense.ID, 10), http.StatusSeeOther)
	}
}

// EditExpense показывает форму редактирования траты
func EditExpense(templates *template.Template, repo *repository.ExpenseRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		expense, ok := findExpense(w, r, repo)
		if !ok {
			return
		}

		render(w, templates, map[string]any{
			"Title": "Редактирование траты",
			"Form": expenseForm{
				Action:  "/expenses/" + strconv.FormatInt(expense.ID, 10) + "/edit",
				Submit:  "Сохранить",
				Cancel:  "/expenses",
				Expense: expense,
			},
		})
	}
}

// UpdateExpense сохраняет изменения траты
func UpdateExpense(repo *repository.ExpenseRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseID(r)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		expense, err := parseExpenseForm(r)
		if err != nil {
			http.Error(w, "Неверные данные формы", http.StatusBadRequest)
			return
		}
		expense.ID = id

		err = repo.Update(expense)
		if errors.Is(err, repository.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			serverError(w, err)
			return
		}

		http.Redirect(w, r, "/expenses?flash=edit&id="+strconv.FormatInt(id, 10), http.StatusSeeOther)
	}
}

// ConfirmDeleteExpense показывает страницу подтверждения удаления
func ConfirmDeleteExpense(templates *template.Template, repo *repository.ExpenseRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		expense, ok := findExpense(w, r, repo)
		if !ok {
			return
		}

		render(w, templates, map[string]any{
			"Title":   "Удаление траты",
			"Expense": expense,
		})
	}
}

// DeleteExpense удаляет трату
func DeleteExpense(repo *repository.ExpenseRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseID(r)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		err = repo.Delete(id)
		if errors.Is(err, repository.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			serverError(w, err)
			return
		}

		http.Redirect(w, r, "/expenses", http.StatusSeeOther)
	}
}

// findExpense достаёт трату по {id} из пути; при ошибке сам пишет ответ и возвращает false
func findExpense(w http.ResponseWriter, r *http.Request, repo *repository.ExpenseRepository) (models.Expense, bool) {
	id, err := parseID(r)
	if err != nil {
		http.NotFound(w, r)
		return models.Expense{}, false
	}

	expense, err := repo.GetByID(id)
	if errors.Is(err, repository.ErrNotFound) {
		http.NotFound(w, r)
		return expense, false
	}
	if err != nil {
		serverError(w, err)
		return expense, false
	}

	return expense, true
}

func render(w http.ResponseWriter, templates *template.Template, data map[string]any) {
	if err := templates.ExecuteTemplate(w, "base", data); err != nil {
		log.Println("Template error:", err)
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

func serverError(w http.ResponseWriter, err error) {
	log.Println("Database error:", err)
	http.Error(w, "Internal Server Error", http.StatusInternalServerError)
}
