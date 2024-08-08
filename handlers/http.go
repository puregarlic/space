package handlers

import (
	"encoding/json"
	"net/http"
)

func SendHttpError(w http.ResponseWriter, status int) {
	http.Error(w, http.StatusText(status), status)
}

func SendJSON(w http.ResponseWriter, code int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(data)
}

func SendErrorJSON(w http.ResponseWriter, code int, err, errDescription string) {
	SendJSON(w, code, map[string]string{
		"error":             err,
		"error_description": errDescription,
	})
}
