package api

import (
	"encoding/json"
	"net/http"
	"strconv"
)

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) error {
	response, err := json.Marshal(&payload)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)

	return nil
}

func respondWithError(w http.ResponseWriter, code int, msg string) error {
	messageBody := map[string]string{
		"status_code":   strconv.Itoa(code),
		"error_message": msg,
	}
	return respondWithJSON(w, code, messageBody)
}
