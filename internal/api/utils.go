package api

import (
	"encoding/json"
	"net/http"
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
	messageBody := ErrorResponse{
		StatusCode:   code,
		ErrorMessage: msg,
	}
	return respondWithJSON(w, code, messageBody)
}

func parseUrlQueryToBool(val string) *bool {
	var parsedVal *bool
	switch val {
	case "true":
		val := true
		parsedVal = &val
	case "false":
		val := false
		parsedVal = &val
	}

	return parsedVal
}
