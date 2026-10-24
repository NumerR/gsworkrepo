package handlers

import (
	"html/template"
	"net/http"
)

func Home(templates *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		render(w, templates, map[string]any{
			"Title": "Главная",
			"Form":  newExpenseForm(),
		})
	}
}
