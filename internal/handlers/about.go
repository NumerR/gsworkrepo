package handlers

import "net/http"

func About(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Учет личных трат."))
}
