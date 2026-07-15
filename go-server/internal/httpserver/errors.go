package httpserver

import (
	"encoding/json"
	"errors"
	"net/http"
)

type appError struct {
	status  int
	message string
}

func (e appError) Error() string {
	return e.message
}

func newAppError(status int, message string) appError {
	return appError{status: status, message: message}
}

func writeError(w http.ResponseWriter, err error) {
	var appErr appError
	if errors.As(err, &appErr) {
		writeJSON(w, appErr.status, map[string]any{"message": appErr.message})
		return
	}
	writeJSON(w, http.StatusInternalServerError, map[string]any{"message": err.Error()})
}

func writeMethodNotAllowed(w http.ResponseWriter) {
	writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"message": "请求方法不支持"})
}

func decodeJSON(req *http.Request, target any) error {
	decoder := json.NewDecoder(req.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}
