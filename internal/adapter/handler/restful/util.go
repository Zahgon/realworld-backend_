package restful

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labasubagia/realworld-backend/internal/core/port"
	"github.com/labasubagia/realworld-backend/internal/core/util/exception"
)

const formatTime string = "2006-01-02T15:04:05.999Z"

func (s *Server) parseToken(r *http.Request) (port.AuthParams, error) {
	authorizationHeader := r.Header.Get(authorizationHeaderKey)
	if len(authorizationHeader) == 0 {
		msg := "authorization header not provided"
		err := exception.New(exception.TypePermissionDenied, msg, nil)
		return port.AuthParams{}, err
	}

	fields := strings.Fields(authorizationHeader)
	if len(fields) < 2 {
		msg := "invalid authorization format"
		err := exception.New(exception.TypePermissionDenied, msg, nil)
		return port.AuthParams{}, err
	}

	authorizationType := strings.ToLower(fields[0])
	if authorizationType != authorizationTypeToken {
		msg := fmt.Sprintf("authorization type %s not supported", authorizationType)
		err := exception.New(exception.TypePermissionDenied, msg, nil)
		return port.AuthParams{}, err
	}

	token := fields[1]
	payload, err := s.service.TokenMaker().VerifyToken(token)
	if err != nil {
		return port.AuthParams{}, err
	}
	return port.AuthParams{Token: token, Payload: payload}, nil
}

func hasToken(r *http.Request) bool {
	authorizationHeader := r.Header.Get(authorizationHeaderKey)
	return len(authorizationHeader) > 0
}

func getAuthArg(r *http.Request) (port.AuthParams, error) {
	arg := r.Context().Value(authorizationArgKey)
	if arg == nil {
		return port.AuthParams{}, exception.New(exception.TypePermissionDenied, "no authorization arguments provided", nil)
	}
	authArg, ok := arg.(port.AuthParams)
	if !ok {
		return port.AuthParams{}, exception.New(exception.TypePermissionDenied, "invalid authorization arguments", nil)
	}
	return authArg, nil
}

func getPagination(r *http.Request) (offset, limit int) {
	query := r.URL.Query()
	offset, err := strconv.Atoi(query.Get("offset"))
	if err != nil {
		offset = 0
	}
	limit, err = strconv.Atoi(query.Get("limit"))
	if err != nil {
		limit = 20
	}
	return offset, limit
}

// clientIP is the address the request is attributed to in the access log. A
// forwarding header wins over the socket address, so a request that came
// through a proxy is logged with the address of the original caller.
func clientIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		if ip := strings.TrimSpace(strings.Split(forwarded, ",")[0]); ip != "" {
			return ip
		}
	}
	if ip := strings.TrimSpace(r.Header.Get("X-Real-Ip")); ip != "" {
		return ip
	}
	if ip, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr)); err == nil {
		return ip
	}
	return strings.TrimSpace(r.RemoteAddr)
}

func timeString(t time.Time) string {
	return t.UTC().Format(formatTime)
}
