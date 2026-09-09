package restful

import (
	"context"
	"net/http"
	"path"
	"regexp"

	"github.com/go-chi/chi/v5"
)

var (
	regSafePrefix         = regexp.MustCompile("[^a-zA-Z0-9/-]+")
	regRemoveRepeatedChar = regexp.MustCompile("/{2,}")
)

// router is the HTTP entry point of the REST adapter. It owns a chi mux and
// adds the two dispatch rules the API has always answered with but that chi
// does not provide itself: the trailing-slash redirect, and the decision of
// what an unmatched request looks like on the wire.
type router struct {
	mux *chi.Mux

	// redirectTrailingSlash answers /path when only /path/ is registered (and
	// the other way round) with a redirect instead of a 404.
	redirectTrailingSlash bool

	// handleMethodNotAllowed stays disabled: a known path addressed with an
	// unregistered method has always answered 404 "page not found", never 405.
	// noMethod is kept so the alternative response is one flag away.
	handleMethodNotAllowed bool

	notFound http.HandlerFunc
	noMethod http.HandlerFunc

	// trees holds the methods that have at least one registered route. A
	// method with no routes at all never redirects, it only ever 404s.
	trees map[string]bool
}

func newRouter(mux *chi.Mux) *router {
	return &router{
		mux:                    mux,
		redirectTrailingSlash:  true,
		handleMethodNotAllowed: false,
		notFound:               notFoundHandler,
		noMethod:               noMethodHandler,
		trees:                  map[string]bool{},
	}
}

// index records which methods carry routes. Called once, after registration.
func (rt *router) index() {
	rt.trees = map[string]bool{}
	_ = chi.Walk(rt.mux, func(method string, route string, handler http.Handler, middlewares ...func(http.Handler) http.Handler) error {
		rt.trees[method] = true
		return nil
	})
}

func (rt *router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rw := newResponseWriter(w)

	if rt.redirectTrailingSlash && !rt.match(r.Method, r.URL.Path) && rt.shouldRedirect(r) {
		// The redirect is decided before the middleware chain, so a redirected
		// request is neither logged nor decorated with CORS headers, and an
		// unauthenticated caller is redirected rather than rejected.
		redirectTrailingSlash(rw, r)
		return
	}

	// Routing runs on the decoded path: chi would otherwise prefer RawPath and
	// treat /articles/%73lug as a different route than /articles/slug.
	rctx := chi.NewRouteContext()
	rctx.RoutePath = r.URL.Path

	// The handler context carries values but neither deadline nor cancellation,
	// so a client that hangs up mid-flight does not abort work already started.
	ctx := context.WithoutCancel(r.Context())
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)

	rt.mux.ServeHTTP(rw, r.WithContext(ctx))

	// A handler may have chosen a status without writing a body.
	rw.WriteHeaderNow()
}

func (rt *router) match(method, routePath string) bool {
	return rt.mux.Match(chi.NewRouteContext(), method, routePath)
}

// shouldRedirect reports whether the unmatched request would match once one
// trailing slash is added or removed.
func (rt *router) shouldRedirect(r *http.Request) bool {
	if r.Method == http.MethodConnect || r.URL.Path == "/" {
		return false
	}
	if !rt.trees[r.Method] {
		return false
	}
	p := r.URL.Path
	if length := len(p); length > 1 && p[length-1] == '/' {
		return rt.match(r.Method, p[:length-1])
	}
	return rt.match(r.Method, p+"/")
}

// methodNotAllowed is where chi reports a known path with an unregistered
// method. With handleMethodNotAllowed disabled it answers the same 404 the
// API has always returned for that case.
func (rt *router) methodNotAllowed(w http.ResponseWriter, r *http.Request) {
	if rt.handleMethodNotAllowed {
		rt.noMethod(w, r)
		return
	}
	rt.notFound(w, r)
}

func notFoundHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusNotFound, map[string]any{"message": "page not found"})
}

func noMethodHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"message": "no method provided"})
}

// redirectTrailingSlash rewrites the request path by adding or removing one
// trailing slash and sends the redirect. X-Forwarded-Prefix is honoured so the
// Location stays valid behind a reverse proxy that strips a path prefix; the
// prefix is stripped of anything outside [a-zA-Z0-9/-] before use.
func redirectTrailingSlash(w *responseWriter, r *http.Request) {
	p := r.URL.Path
	if prefix := path.Clean(r.Header.Get("X-Forwarded-Prefix")); prefix != "." {
		prefix = regSafePrefix.ReplaceAllString(prefix, "")
		prefix = regRemoveRepeatedChar.ReplaceAllString(prefix, "/")
		p = prefix + "/" + r.URL.Path
	}
	r.URL.Path = p + "/"
	if length := len(p); length > 1 && p[length-1] == '/' {
		r.URL.Path = p[:length-1]
	}
	redirectRequest(w, r)
}

// redirectRequest sends the redirect itself. GET is permanent, anything else
// is temporary so the method and body survive the second request. The target
// is taken from URL.String(), which keeps the query string.
func redirectRequest(w *responseWriter, r *http.Request) {
	code := http.StatusMovedPermanently
	if r.Method != http.MethodGet {
		code = http.StatusTemporaryRedirect
	}
	http.Redirect(w, r, r.URL.String(), code)
	w.WriteHeaderNow()
}
