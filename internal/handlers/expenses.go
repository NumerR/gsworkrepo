package handlers

import (
	"errors"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strconv"

	"gdwork/internal/models"
	"gdwork/internal/repository"
)

// ExpenseList показывает траты текущего пользователя, отфильтрованные по query-параметрам
func ExpenseList(
	templates *template.Template,
	repo *repository.ExpenseRepository,
	categories *repository.CategoryRepository,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := currentUser(r)

		categoryList, err := categories.List()
		if err != nil {
			serverError(w, err)
			return
		}

		filter := parseExpenseFilter(r)

		expenses, err := repo.List(user.ID, filter.Filter())
		if err != nil {
			serverError(w, err)
			return
		}

		form := newExpenseForm(categoryList, csrfToken(r))
		// форма добавления стоит на этом же списке, поэтому сохраняем фильтр в action
		form.Action = "/expenses" + filter.Query()

		render(w, r, templates, map[string]any{
			"Title":      "Траты",
			"Expenses":   expenses,
			"Categories": categoryList,
			"Filter":     filter,
			"Form":       form,
		})
	}
}

// AddExpense принимает данные из формы и сохраняет трату текущего пользователя
func AddExpense(repo *repository.ExpenseRepository, categories *repository.CategoryRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		expense, err := parseExpenseForm(r)
		if err != nil {
			http.Error(w, "Неверные данные формы", http.StatusBadRequest)
			return
		}
		expense.UserID = currentUser(r).ID

		known, err := checkCategory(categories, expense.CategoryID)
		if err != nil {
			serverError(w, err)
			return
		}
		if !known {
			http.Error(w, "Неверные данные формы", http.StatusBadRequest)
			return
		}

		if err := repo.Create(&expense); err != nil {
			serverError(w, err)
			return
		}

		redirectToList(w, r, url.Values{
			"flash": {"add"},
			"id":    {strconv.FormatInt(expense.ID, 10)},
		})
	}
}

// EditExpense показывает форму редактирования траты
func EditExpense(
	templates *template.Template,
	repo *repository.ExpenseRepository,
	categories *repository.CategoryRepository,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		categoryList, err := categories.List()
		if err != nil {
			serverError(w, err)
			return
		}

		expense, ok := findExpense(w, r, repo)
		if !ok {
			return
		}

		// фильтры прилетели в ссылке со списка — держим их в action и «Отмене»,
		// чтобы после сохранения вернуться в тот же срез
		filter := parseExpenseFilter(r)

		render(w, r, templates, map[string]any{
			"Title": "Редактирование траты",
			"Form": expenseForm{
				Action:     "/expenses/" + strconv.FormatInt(expense.ID, 10) + "/edit" + filter.Query(),
				Submit:     "Сохранить",
				Cancel:     "/expenses" + filter.Query(),
				Expense:    expense,
				Categories: categoryList,
				CSRF:       csrfToken(r),
			},
		})
	}
}

// UpdateExpense сохраняет изменения траты
func UpdateExpense(repo *repository.ExpenseRepository, categories *repository.CategoryRepository) http.HandlerFunc {
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

		known, err := checkCategory(categories, expense.CategoryID)
		if err != nil {
			serverError(w, err)
			return
		}
		if !known {
			http.Error(w, "Неверные данные формы", http.StatusBadRequest)
			return
		}

		expense.ID = id
		// Обновление идёт по WHERE id = ? AND user_id = ?, поэтому чужая трата
		// не изменится: запрос затронет ноль строк, и это выглядит как 404.
		expense.UserID = currentUser(r).ID

		err = repo.Update(expense)
		if errors.Is(err, repository.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			serverError(w, err)
			return
		}

		redirectToList(w, r, url.Values{
			"flash": {"edit"},
			"id":    {strconv.FormatInt(id, 10)},
		})
	}
}

// ConfirmDeleteExpense показывает страницу подтверждения удаления
func ConfirmDeleteExpense(templates *template.Template, repo *repository.ExpenseRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		expense, ok := findExpense(w, r, repo)
		if !ok {
			return
		}

		render(w, r, templates, map[string]any{
			"Title":   "Удаление траты",
			"Expense": expense,
			"Filter":  parseExpenseFilter(r),
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

		err = repo.Delete(id, currentUser(r).ID)
		if errors.Is(err, repository.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			serverError(w, err)
			return
		}

		redirectToList(w, r, nil)
	}
}

// redirectToList возвращает в список трат, сохраняя фильтры из текущего запроса
// и добавляя служебные параметры вроде flash и id
func redirectToList(w http.ResponseWriter, r *http.Request, extra url.Values) {
	query := parseExpenseFilter(r).Values()
	for key, values := range extra {
		query[key] = values
	}

	location := "/expenses"
	if encoded := query.Encode(); encoded != "" {
		location += "?" + encoded
	}

	http.Redirect(w, r, location, http.StatusSeeOther)
}

// checkCategory говорит, можно ли сохранить трату с такой категорией:
// 0 — «без категории», которого нет в справочнике — ошибка данных формы.
func checkCategory(categories *repository.CategoryRepository, id int64) (bool, error) {
	if id == 0 {
		return true, nil
	}
	return categories.Exists(id)
}

// findExpense достаёт трату по {id} из пути; при ошибке сам пишет ответ и возвращает false
func findExpense(w http.ResponseWriter, r *http.Request, repo *repository.ExpenseRepository) (models.Expense, bool) {
	id, err := parseID(r)
	if err != nil {
		http.NotFound(w, r)
		return models.Expense{}, false
	}

	expense, err := repo.GetByID(id, currentUser(r).ID)
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

// render дописывает в данные авторизованного пользователя и CSRF-токен: оба
// нужны на каждой странице, а не только там, где открыта своя форма.
// Приватная страница не должна оседать в кэше браузера или прокси (V4.1.1, V6.1.2).
func render(w http.ResponseWriter, r *http.Request, templates *template.Template, data map[string]any) {
	user := currentUser(r)
	data["User"] = user
	data["CSRF"] = csrfToken(r)
	if user.ID != 0 {
		noStore(w)
	}

	if err := templates.ExecuteTemplate(w, "base", data); err != nil {
		log.Println("Template error:", err)
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

func serverError(w http.ResponseWriter, err error) {
	log.Println("Database error:", err)
	http.Error(w, "Internal Server Error", http.StatusInternalServerError)
}
