package handlers

import (
	"html/template"
	"log"
	"net/http"
)

func Home(templates *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		err := templates.ExecuteTemplate(w, "base", map[string]any{
			"Title": "Test",
		})

		if err != nil {
			log.Println("Template error:", err)
			http.Error(w, "Template error", http.StatusInternalServerError)
			return
		}
	}
}
