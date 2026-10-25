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

		categoryList, err := categories.List()
		if err != nil {
			serverError(w, err)
			return
		}

		render(w, templates, map[string]any{
			"Title": "Главная",
			"Form":  newExpenseForm(categoryList),
		})
	}
}
