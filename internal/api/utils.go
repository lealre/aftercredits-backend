package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

var ErrForbidden = errors.New("you do not have permission to perform this action")

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

func respondWithForbidden(w http.ResponseWriter) error {
	statusCode := http.StatusForbidden
	messageBody := ErrorResponse{
		StatusCode:   statusCode,
		ErrorMessage: formatErrorMessage(ErrForbidden),
	}
	return respondWithJSON(w, statusCode, messageBody)
}

func RespondWithUnauthorized(w http.ResponseWriter, err error) error {
	statusCode := http.StatusUnauthorized
	messageBody := ErrorResponse{
		StatusCode:   statusCode,
		ErrorMessage: formatErrorMessage(err),
	}
	return respondWithJSON(w, statusCode, messageBody)
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

func formatErrorMessage(err error) string {
	errorMsg := err.Error()
	if len(errorMsg) > 0 {
		return strings.ToUpper(errorMsg[:1]) + errorMsg[1:]
	}
	return ""
}
