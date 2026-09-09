package restful

import (
	"context"
	"net/http"
)

// ctxKey is the type of the values this package stores on the request context.
type ctxKey string

const (
	authorizationHeaderKey = "authorization"
	authorizationTypeToken = "token"
	authorizationArgKey    = ctxKey("authorization_arg")
)

// AuthMiddleware resolves the bearer of the request. A caller that sent an
// Authorization header is always rejected when that header does not check out.
// A caller that sent none is only rejected when autoDenied is set; otherwise
// the request proceeds and the zero-value credentials are stored, which is why
// the optional-auth handlers see an empty AuthParams rather than an error.
func (server *Server) AuthMiddleware(autoDenied bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authArg, err := server.parseToken(r)
			if err != nil {
				if hasToken(r) {
					errorHandler(w, err)
					return
				}
				if autoDenied {
					errorHandler(w, err)
					return
				}
			}
			ctx := context.WithValue(r.Context(), authorizationArgKey, authArg)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
