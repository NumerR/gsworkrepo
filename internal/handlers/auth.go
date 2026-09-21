package handlers

import (
	"html/template"
	"net/http"
)

func Login(templates *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		err := templates.ExecuteTemplate(w, "base", map[string]any{
			"Title": "Вход",
		})

		if err != nil {
			http.Error(w, "Template error", http.StatusInternalServerError)
			return
		}
	}
}

func Register(templates *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		if r.Method == http.MethodGet {

			err := templates.ExecuteTemplate(w, "base", map[string]any{
				"Title": "Регистрация",
			})

			if err != nil {
				http.Error(w, "Template error", http.StatusInternalServerError)
			}

			return
		}

		if r.Method == http.MethodPost {

			err := r.ParseForm()
			if err != nil {
				http.Error(w, "Invalid form", http.StatusBadRequest)
				return
			}

			name := r.FormValue("name")
			email := r.FormValue("email")

			logMessage := "Registration: " + name + " / " + email

			println(logMessage)

			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}

		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
