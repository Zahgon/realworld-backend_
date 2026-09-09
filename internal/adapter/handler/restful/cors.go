package restful

import "net/http"

// Cross-origin defaults: every origin is allowed, credentials are not, and the
// preflight answer is cacheable for twelve hours.
const (
	corsAllowOrigin  = "*"
	corsAllowMethods = "GET,POST,PUT,PATCH,DELETE,HEAD,OPTIONS"
	corsAllowHeaders = "Origin,Content-Length,Content-Type"
	corsMaxAge       = "43200"
)

// cors answers cross-origin requests. A request without an Origin, or one whose
// Origin is this very host, is not cross-origin and passes through untouched --
// in particular it gets no Access-Control-Allow-Origin header. A preflight is
// answered here and never reaches the router, which is why OPTIONS returns 204
// even for a path that does not exist.
func cors() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if len(origin) == 0 {
				next.ServeHTTP(w, r)
				return
			}
			host := r.Host
			if origin == "http://"+host || origin == "https://"+host {
				next.ServeHTTP(w, r)
				return
			}

			if r.Method == http.MethodOptions {
				header := w.Header()
				header.Set("Access-Control-Allow-Origin", corsAllowOrigin)
				header.Set("Access-Control-Allow-Methods", corsAllowMethods)
				header.Set("Access-Control-Allow-Headers", corsAllowHeaders)
				header.Set("Access-Control-Max-Age", corsMaxAge)
				abortWithStatus(w, http.StatusNoContent)
				return
			}

			w.Header().Set("Access-Control-Allow-Origin", corsAllowOrigin)
			next.ServeHTTP(w, r)
		})
	}
}
