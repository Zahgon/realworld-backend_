package restful

import (
	"net/http"

	"github.com/labasubagia/realworld-backend/internal/core/util/exception"
)

func errorHandler(w http.ResponseWriter, err error) {
	if err == nil {
		writeJSON(w, http.StatusOK, nil)
		return
	}
	fail, ok := err.(*exception.Exception)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, fail)
		return
	}
	if !fail.HasError() {
		fail.AddError("exception", fail.Message)
	}
	var statusCode int
	switch fail.Type {
	case exception.TypeNotFound:
		statusCode = http.StatusNotFound
	case exception.TypeTokenExpired, exception.TypeTokenInvalid, exception.TypePermissionDenied:
		statusCode = http.StatusUnauthorized
	case exception.TypeValidation:
		statusCode = http.StatusUnprocessableEntity
	default:
		statusCode = http.StatusInternalServerError
	}
	writeJSON(w, statusCode, fail)
}
