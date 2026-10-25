package handlers

import (
	"html/template"
	"net/http"

	"gdwork/internal/repository"
)

func Home(templates *template.Template, categories *repository.CategoryRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		// Форма траты гостю ни к чему: сохранение доступно только владельцу списка
		data := map[string]any{"Title": "Главная"}

		if currentUser(r).ID != 0 {
			categoryList, err := categories.List()
			if err != nil {
				serverError(w, err)
				return
			}
			data["Form"] = newExpenseForm(categoryList, csrfToken(r))
		}

		render(w, r, templates, data)
	}
}
