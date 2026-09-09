package restful

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/labasubagia/realworld-backend/internal/core/domain"
	"github.com/labasubagia/realworld-backend/internal/core/port"
	"github.com/labasubagia/realworld-backend/internal/core/util/exception"
	"github.com/stretchr/testify/require"
)

// TestJSONResponseEncoding pins the two serializer defaults the public
// interface depends on: the exact Content-Type spelling and the absence of a
// trailing newline after the JSON document.
func TestJSONResponseEncoding(t *testing.T) {
	h := newTestHarness(t)

	res := h.get("/", nil)
	require.Equal(t, http.StatusOK, res.status)
	require.Equal(t, "application/json; charset=utf-8", res.header.Get("Content-Type"))
	require.Equal(t, `{"message":"Hello World!"}`, res.body)
	require.False(t, strings.HasSuffix(res.body, "\n"), "JSON responses must not end with a newline")
	require.Equal(t, strconv.Itoa(len(res.body)), res.header.Get("Content-Length"))
}

func TestRootHandler(t *testing.T) {
	h := newTestHarness(t)

	res := h.get("/", nil)
	require.Equal(t, http.StatusOK, res.status)
	require.Equal(t, `{"message":"Hello World!"}`, res.body)
	require.Equal(t, "26", res.header.Get("Content-Length"))
	require.Equal(t, strconv.Itoa(len(res.body)), res.header.Get("Content-Length"))
}

// TestUnknownRoute covers the NoRoute handler.
func TestUnknownRoute(t *testing.T) {
	h := newTestHarness(t)

	res := h.get("/nope", nil)
	require.Equal(t, http.StatusNotFound, res.status)
	require.Equal(t, `{"message":"page not found"}`, res.body)
	require.Equal(t, "application/json; charset=utf-8", res.header.Get("Content-Type"))
	require.Equal(t, "28", res.header.Get("Content-Length"))
}

// TestUnknownMethodIsNotFound is the "native 405 leaking" guard. A request that
// uses a method the path does not serve must answer 404 with the NoRoute body,
// and must not advertise an Allow header.
func TestUnknownMethodIsNotFound(t *testing.T) {
	h := newTestHarness(t)

	testCases := []struct {
		name   string
		method string
		path   string
	}{
		{name: "POST root", method: http.MethodPost, path: "/"},
		{name: "DELETE root", method: http.MethodDelete, path: "/"},
		{name: "GET users", method: http.MethodGet, path: "/users"},
		{name: "PUT tags", method: http.MethodPut, path: "/tags/"},
		{name: "PATCH articles", method: http.MethodPatch, path: "/articles/"},
		{name: "PROPFIND tags", method: "PROPFIND", path: "/tags/"},
		{name: "TRACE tags", method: http.MethodTrace, path: "/tags/"},
		{name: "unregistered verb", method: "FOO", path: "/tags/"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			res := h.do(tc.method, tc.path, "", nil)
			require.Equal(t, http.StatusNotFound, res.status)
			require.Equal(t, `{"message":"page not found"}`, res.body)
			require.Empty(t, res.header.Get("Allow"), "no Allow header may leak to the client")
		})
	}
}

// TestTrailingSlashRedirect pins the redirect codes and the Location targets
// produced when a request misses only by a trailing slash.
func TestTrailingSlashRedirect(t *testing.T) {
	h := newTestHarness(t)

	testCases := []struct {
		name     string
		method   string
		path     string
		status   int
		location string
	}{
		{name: "add slash on GET", method: http.MethodGet, path: "/user", status: http.StatusMovedPermanently, location: "/user/"},
		{name: "add slash on PUT", method: http.MethodPut, path: "/user", status: http.StatusTemporaryRedirect, location: "/user/"},
		{name: "add slash on articles", method: http.MethodGet, path: "/articles", status: http.StatusMovedPermanently, location: "/articles/"},
		{name: "add slash on tags", method: http.MethodGet, path: "/tags", status: http.StatusMovedPermanently, location: "/tags/"},
		{name: "strip slash on POST", method: http.MethodPost, path: "/users/", status: http.StatusTemporaryRedirect, location: "/users"},
		{name: "strip slash on feed", method: http.MethodGet, path: "/articles/feed/", status: http.StatusMovedPermanently, location: "/articles/feed"},
		{name: "strip slash on slug", method: http.MethodGet, path: "/articles/some-slug/", status: http.StatusMovedPermanently, location: "/articles/some-slug"},
		{name: "collapse repeated slash", method: http.MethodGet, path: "/articles//", status: http.StatusMovedPermanently, location: "/articles/"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			res := h.do(tc.method, tc.path, "", nil)
			require.Equal(t, tc.status, res.status)
			require.Equal(t, tc.location, res.header.Get("Location"))
		})
	}

	// The redirect body is written by net/http, not by a handler, so it is easy
	// to lose: a GET carries the anchor document, anything else carries nothing.
	get := h.do(http.MethodGet, "/user", "", nil)
	require.Equal(t, "<a href=\"/user/\">Moved Permanently</a>.\n\n", get.body)
	require.Equal(t, "text/html; charset=utf-8", get.header.Get("Content-Type"))
	put := h.do(http.MethodPut, "/user", "", nil)
	require.Empty(t, put.body)
	require.Empty(t, put.header.Get("Content-Type"))
	require.Equal(t, "0", put.header.Get("Content-Length"))
}

// TestNoRedirectWithoutMatchingRoute proves the redirect only fires when the
// alternate path is served by the same method.
func TestNoRedirectWithoutMatchingRoute(t *testing.T) {
	h := newTestHarness(t)

	for _, path := range []string{"/users", "/users/login", "/tags/extra", "/articles/some-slug/nonsense"} {
		t.Run(path, func(t *testing.T) {
			res := h.get(path, nil)
			require.Equal(t, http.StatusNotFound, res.status)
			require.Equal(t, `{"message":"page not found"}`, res.body)
			require.Empty(t, res.header.Get("Location"))
		})
	}
}

// TestGuardDoesNotRunOnUnmatchedPath keeps the authorization middleware scoped
// to the routes it guards. A miss below a guarded prefix is a 404, never a 401.
func TestGuardDoesNotRunOnUnmatchedPath(t *testing.T) {
	h := newTestHarness(t)

	for _, path := range []string{"/user/nonsense", "/profiles/alice/nonsense", "/articles/some-slug/favorite/extra"} {
		t.Run(path, func(t *testing.T) {
			res := h.get(path, nil)
			require.Equal(t, http.StatusNotFound, res.status)
			require.Equal(t, `{"message":"page not found"}`, res.body)
		})
	}
}

// TestCORSPreflight pins the exact header set emitted for a preflight request,
// including the headers that must be absent.
func TestCORSPreflight(t *testing.T) {
	h := newTestHarness(t)

	res := h.do(http.MethodOptions, "/articles/", "", map[string]string{
		"Origin":                         "http://example.com",
		"Access-Control-Request-Method":  http.MethodPost,
		"Access-Control-Request-Headers": "content-type",
	})
	require.Equal(t, http.StatusNoContent, res.status)
	require.Equal(t, "*", res.header.Get("Access-Control-Allow-Origin"))
	require.Equal(t, "Origin,Content-Length,Content-Type", res.header.Get("Access-Control-Allow-Headers"))
	require.Equal(t, "GET,POST,PUT,PATCH,DELETE,HEAD,OPTIONS", res.header.Get("Access-Control-Allow-Methods"))
	require.Equal(t, "43200", res.header.Get("Access-Control-Max-Age"))
	require.Empty(t, res.header.Get("Vary"))
	require.Empty(t, res.header.Get("Content-Type"))
	require.Empty(t, res.body)
	require.Empty(t, res.header.Get("Content-Length"))
}

// TestCORSPreflightOnUnknownRoute shows the preflight answer does not depend on
// the path being routable.
func TestCORSPreflightOnUnknownRoute(t *testing.T) {
	h := newTestHarness(t)

	res := h.do(http.MethodOptions, "/nope", "", map[string]string{
		"Origin":                        "http://example.com",
		"Access-Control-Request-Method": http.MethodGet,
	})
	require.Equal(t, http.StatusNoContent, res.status)
	require.Equal(t, "*", res.header.Get("Access-Control-Allow-Origin"))
}

// TestCORSSimpleRequest checks that a plain cross-origin request carries the
// allow-origin header and nothing more.
func TestCORSSimpleRequest(t *testing.T) {
	h := newTestHarness(t)

	res := h.get("/tags/", map[string]string{"Origin": "http://example.com"})
	require.Equal(t, http.StatusOK, res.status)
	require.Equal(t, "*", res.header.Get("Access-Control-Allow-Origin"))
	require.Empty(t, res.header.Get("Access-Control-Allow-Methods"))
	require.Empty(t, res.header.Get("Vary"))
	require.Empty(t, res.header.Get("Access-Control-Allow-Headers"))
	require.Empty(t, res.header.Get("Access-Control-Max-Age"))
}

// TestCORSSkippedForSameOrigin covers the branch where the request originates
// from the server's own origin.
func TestCORSSkippedForSameOrigin(t *testing.T) {
	h := newTestHarness(t)

	res := h.get("/tags/", map[string]string{"Origin": h.http.URL})
	require.Equal(t, http.StatusOK, res.status)
	require.Empty(t, res.header.Get("Access-Control-Allow-Origin"))
}

// TestOptionsWithoutOriginIsNotFound shows OPTIONS is not a routed method.
func TestOptionsWithoutOriginIsNotFound(t *testing.T) {
	h := newTestHarness(t)

	res := h.do(http.MethodOptions, "/tags/", "", nil)
	require.Equal(t, http.StatusNotFound, res.status)
	require.Equal(t, `{"message":"page not found"}`, res.body)
}

// TestAuthorizationErrors pins every rejection message the authorization
// middleware can produce.
func TestAuthorizationErrors(t *testing.T) {
	h := newTestHarness(t)

	testCases := []struct {
		name    string
		path    string
		headers map[string]string
		body    string
	}{
		{
			name: "missing header",
			path: "/user/",
			body: `{"errors":{"exception":["authorization header not provided"]}}`,
		},
		{
			name:    "single field",
			path:    "/user/",
			headers: map[string]string{"Authorization": "token"},
			body:    `{"errors":{"exception":["invalid authorization format"]}}`,
		},
		{
			name:    "unsupported type",
			path:    "/user/",
			headers: map[string]string{"Authorization": "Bearer abc"},
			body:    `{"errors":{"exception":["authorization type bearer not supported"]}}`,
		},
		{
			name:    "invalid token",
			path:    "/user/",
			headers: map[string]string{"Authorization": "Token garbage"},
			body:    `{"errors":{"exception":["invalid token"]}}`,
		},
		{
			name:    "required guard on feed",
			path:    "/articles/feed",
			headers: map[string]string{"Authorization": "Token garbage"},
			body:    `{"errors":{"exception":["invalid token"]}}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			res := h.get(tc.path, tc.headers)
			require.Equal(t, http.StatusUnauthorized, res.status)
			require.Equal(t, tc.body, res.body)
			require.Equal(t, "application/json; charset=utf-8", res.header.Get("Content-Type"))
		})
	}
}

// TestOptionalGuardAllowsAnonymous proves the optional guard lets a request
// through when no credentials are supplied.
func TestOptionalGuardAllowsAnonymous(t *testing.T) {
	h := newTestHarness(t)

	var seen port.AuthParams
	h.service.article.get = func(_ context.Context, arg port.GetArticleParams) (domain.Article, error) {
		seen = arg.AuthArg
		return testArticle(), nil
	}

	res := h.get("/articles/how-to-train-your-dragon", nil)
	require.Equal(t, http.StatusOK, res.status)
	require.Equal(t, port.AuthParams{}, seen)
}

// TestAuthorizedRequestReachesService checks the happy path of the required
// guard and the payload the handler forwards.
func TestAuthorizedRequestReachesService(t *testing.T) {
	h := newTestHarness(t)

	var seen port.AuthParams
	h.service.user.current = func(_ context.Context, arg port.AuthParams) (domain.User, error) {
		seen = arg
		return testUser(), nil
	}

	value := h.token()
	res := h.get("/user/", authHeader(value))
	require.Equal(t, http.StatusOK, res.status)
	require.Equal(t, value, seen.Token)
	require.Equal(t, testUser().ID, seen.Payload.UserID)
	require.Equal(
		t,
		`{"user":{"email":"alice@realworld.test","username":"alice","bio":"bio","image":"`+domain.UserDefaultImage+`","token":"test-token"}}`,
		res.body,
	)
}

// TestSubLoggerReachesRequestContext proves the logging middleware publishes
// its per-request logger where the service layer looks for it.
func TestSubLoggerReachesRequestContext(t *testing.T) {
	h := newTestHarness(t)

	var found bool
	h.service.article.listTags = func(ctx context.Context) ([]string, error) {
		_, found = ctx.Value(port.SubLoggerCtxKey).(port.Logger)
		return []string{}, nil
	}

	res := h.get("/tags/", nil)
	require.Equal(t, http.StatusOK, res.status)
	require.True(t, found, "handlers must forward a context carrying the request sub logger")
}

// TestBindFailure documents the response produced when the request body is not
// valid JSON. The status line is committed before the error body is written,
// so the payload is the JSON literal null under a sniffed content type.
func TestBindFailure(t *testing.T) {
	h := newTestHarness(t)

	for _, body := range []string{`{bad`, ``, `[]`, `{"user":{"email":1}}`} {
		t.Run(body, func(t *testing.T) {
			res := h.do(http.MethodPost, "/users", body, map[string]string{"Content-Type": "application/json"})
			require.Equal(t, http.StatusBadRequest, res.status)
			require.Equal(t, "null", res.body)
			require.Equal(t, "text/plain; charset=utf-8", res.header.Get("Content-Type"))
		})
	}

	res := h.do(http.MethodPost, "/users", `{bad`, map[string]string{"Content-Type": "application/json"})
	require.Equal(t, "4", res.header.Get("Content-Length"))
	require.Empty(t, res.header.Get("Access-Control-Allow-Origin"))
}

// TestBindIgnoresContentType shows the body is decoded as JSON whatever the
// declared media type is, so no request is rejected as unsupported media.
func TestBindIgnoresContentType(t *testing.T) {
	h := newTestHarness(t)

	var seen port.RegisterParams
	h.service.user.register = func(_ context.Context, arg port.RegisterParams) (domain.User, error) {
		seen = arg
		return testUser(), nil
	}

	body := `{"user":{"email":"alice@realworld.test","username":"alice","password":"secret123"}}`
	res := h.do(http.MethodPost, "/users", body, map[string]string{"Content-Type": "text/plain"})
	require.Equal(t, http.StatusCreated, res.status)
	require.Equal(t, "alice@realworld.test", seen.User.Email)
	require.Equal(t, "alice", seen.User.Username)
	require.Equal(t, "secret123", seen.User.Password)
}

// TestErrorStatusMapping pins the translation from exception kind to status.
func TestErrorStatusMapping(t *testing.T) {
	testCases := []struct {
		name   string
		kind   string
		status int
	}{
		{name: "not found", kind: exception.TypeNotFound, status: http.StatusNotFound},
		{name: "token expired", kind: exception.TypeTokenExpired, status: http.StatusUnauthorized},
		{name: "token invalid", kind: exception.TypeTokenInvalid, status: http.StatusUnauthorized},
		{name: "permission denied", kind: exception.TypePermissionDenied, status: http.StatusUnauthorized},
		{name: "validation", kind: exception.TypeValidation, status: http.StatusUnprocessableEntity},
		{name: "internal", kind: exception.TypeInternal, status: http.StatusInternalServerError},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			h := newTestHarness(t)
			h.service.article.listTags = func(context.Context) ([]string, error) {
				return nil, exception.New(tc.kind, "boom", nil)
			}

			res := h.get("/tags/", nil)
			require.Equal(t, tc.status, res.status)
			require.Equal(t, `{"errors":{"exception":["boom"]}}`, res.body)
		})
	}
}

// TestErrorHandlerWithForeignError covers the branch that receives an error the
// handler layer does not own.
func TestErrorHandlerWithForeignError(t *testing.T) {
	h := newTestHarness(t)
	h.service.article.listTags = func(context.Context) ([]string, error) {
		return nil, context.Canceled
	}

	res := h.get("/tags/", nil)
	require.Equal(t, http.StatusInternalServerError, res.status)
	require.Equal(t, "null", res.body)
}

// TestPercentEncodedPath shows routing happens on the decoded path.
func TestPercentEncodedPath(t *testing.T) {
	h := newTestHarness(t)

	var seen string
	h.service.user.profile = func(_ context.Context, arg port.ProfileParams) (domain.User, error) {
		seen = arg.Username
		return testUser(), nil
	}

	res := h.get("/profiles/al%69ce/", nil)
	require.Equal(t, http.StatusOK, res.status)
	require.Equal(t, "alice", seen)

	res = h.get("/profiles/a%2Fb/", nil)
	require.Equal(t, http.StatusNotFound, res.status)
}

// TestPagination pins the query defaults and the fallback applied to values
// that do not parse.
func TestPagination(t *testing.T) {
	h := newTestHarness(t)

	testCases := []struct {
		name   string
		query  string
		offset int
		limit  int
	}{
		{name: "defaults", query: "", offset: 0, limit: 20},
		{name: "explicit", query: "?offset=5&limit=3", offset: 5, limit: 3},
		{name: "unparsable", query: "?offset=xyz&limit=abc", offset: 0, limit: 20},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var seen port.ListArticleParams
			h.service.article.list = func(_ context.Context, arg port.ListArticleParams) ([]domain.Article, error) {
				seen = arg
				return nil, nil
			}

			res := h.get("/articles/"+tc.query, nil)
			require.Equal(t, http.StatusOK, res.status)
			require.Equal(t, tc.offset, seen.Offset)
			require.Equal(t, tc.limit, seen.Limit)
		})
	}
}

// TestListArticleFilters checks the query parameters the handler forwards.
func TestListArticleFilters(t *testing.T) {
	h := newTestHarness(t)

	var seen port.ListArticleParams
	h.service.article.list = func(_ context.Context, arg port.ListArticleParams) ([]domain.Article, error) {
		seen = arg
		return []domain.Article{testArticle()}, nil
	}

	res := h.get("/articles/?tag=dragons&author=alice&favorited=bob", nil)
	require.Equal(t, http.StatusOK, res.status)
	require.Equal(t, []string{"dragons"}, seen.Tags)
	require.Equal(t, []string{"alice"}, seen.AuthorNames)
	require.Equal(t, []string{"bob"}, seen.FavoritedNames)
	require.Contains(t, res.body, `"articlesCount":1`)
}

// TestEmptyCollectionsSerializeAsArrays guards the empty-slice defaults the
// clients rely on.
func TestEmptyCollectionsSerializeAsArrays(t *testing.T) {
	h := newTestHarness(t)

	res := h.get("/articles/", nil)
	require.Equal(t, http.StatusOK, res.status)
	require.Equal(t, `{"articles":[],"articlesCount":0}`, res.body)

	res = h.get("/articles/how-to-train-your-dragon/comments/", nil)
	require.Equal(t, http.StatusOK, res.status)
	require.Equal(t, `{"comments":[]}`, res.body)

	res = h.get("/tags/", nil)
	require.Equal(t, http.StatusOK, res.status)
	require.Equal(t, `{"tags":null}`, res.body)

	// An article with no tags must still serialise "tagList":[]. Measured on the
	// running Gin baseline: POST /articles/ with no tagList field answered
	// {"article":{"slug":"tagless-probe",...,"tagList":[],...}}, never null.
	tagless := testArticle()
	tagless.TagNames = nil
	h.service.article.get = func(context.Context, port.GetArticleParams) (domain.Article, error) {
		return tagless, nil
	}
	res = h.get("/articles/how-to-train-your-dragon", nil)
	require.Equal(t, http.StatusOK, res.status)
	require.Contains(t, res.body, `"tagList":[]`)
	require.NotContains(t, res.body, `"tagList":null`)
}

// TestRouteTable walks every registered endpoint and pins its method, path and
// success status.
func TestRouteTable(t *testing.T) {
	h := newTestHarness(t)
	value := h.token()

	body := `{"user":{"email":"alice@realworld.test","username":"alice","password":"secret123"},` +
		`"article":{"title":"How to train your dragon","description":"d","body":"b","tagList":["dragons"]},` +
		`"comment":{"body":"His name was my name too."}}`

	testCases := []struct {
		name   string
		method string
		path   string
		auth   bool
		status int
		// prefix pins which handler answered. Without it the table only proves
		// that some handler returned the right status, so two routes wired to
		// each other's handler would still pass.
		prefix string
	}{
		{name: "register", method: http.MethodPost, path: "/users", status: http.StatusCreated, prefix: `{"user":`},
		{name: "login", method: http.MethodPost, path: "/users/login", status: http.StatusOK, prefix: `{"user":`},
		{name: "current user", method: http.MethodGet, path: "/user/", auth: true, status: http.StatusOK, prefix: `{"user":`},
		{name: "update user", method: http.MethodPut, path: "/user/", auth: true, status: http.StatusOK, prefix: `{"user":`},
		{name: "profile", method: http.MethodGet, path: "/profiles/alice/", status: http.StatusOK, prefix: `{"profile":`},
		{name: "follow", method: http.MethodPost, path: "/profiles/alice/follow", auth: true, status: http.StatusOK, prefix: `{"profile":`},
		{name: "unfollow", method: http.MethodDelete, path: "/profiles/alice/follow", auth: true, status: http.StatusOK, prefix: `{"profile":`},
		{name: "list articles", method: http.MethodGet, path: "/articles/", status: http.StatusOK, prefix: `{"articles":`},
		{name: "feed", method: http.MethodGet, path: "/articles/feed", auth: true, status: http.StatusOK, prefix: `{"articles":`},
		{name: "get article", method: http.MethodGet, path: "/articles/how-to-train-your-dragon", status: http.StatusOK, prefix: `{"article":`},
		{name: "create article", method: http.MethodPost, path: "/articles/", auth: true, status: http.StatusCreated, prefix: `{"article":`},
		{name: "update article", method: http.MethodPut, path: "/articles/how-to-train-your-dragon", auth: true, status: http.StatusOK, prefix: `{"article":`},
		{name: "delete article", method: http.MethodDelete, path: "/articles/how-to-train-your-dragon", auth: true, status: http.StatusOK, prefix: `{"status":"OK"}`},
		{name: "add comment", method: http.MethodPost, path: "/articles/how-to-train-your-dragon/comments/", auth: true, status: http.StatusOK, prefix: `{"comment":`},
		{name: "list comments", method: http.MethodGet, path: "/articles/how-to-train-your-dragon/comments/", status: http.StatusOK, prefix: `{"comments":`},
		{name: "favorite", method: http.MethodPost, path: "/articles/how-to-train-your-dragon/favorite/", auth: true, status: http.StatusOK, prefix: `{"article":`},
		{name: "unfavorite", method: http.MethodDelete, path: "/articles/how-to-train-your-dragon/favorite/", auth: true, status: http.StatusOK, prefix: `{"article":`},
		{name: "tags", method: http.MethodGet, path: "/tags/", status: http.StatusOK, prefix: `{"tags":`},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			headers := map[string]string{"Content-Type": "application/json"}
			if tc.auth {
				headers["Authorization"] = "Token " + value
			}
			res := h.do(tc.method, tc.path, body, headers)
			require.Equal(t, tc.status, res.status)
			require.Equal(t, "application/json; charset=utf-8", res.header.Get("Content-Type"))
			require.True(t, strings.HasPrefix(res.body, tc.prefix), "route %s %s answered %s", tc.method, tc.path, res.body)
		})
	}
}

// TestDeleteResponses pins the acknowledgement payload the delete endpoints
// return.
func TestDeleteResponses(t *testing.T) {
	h := newTestHarness(t)
	value := h.token()

	res := h.do(http.MethodDelete, "/articles/how-to-train-your-dragon", "", authHeader(value))
	require.Equal(t, http.StatusOK, res.status)
	require.Equal(t, `{"status":"OK"}`, res.body)

	res = h.do(
		http.MethodDelete,
		"/articles/how-to-train-your-dragon/comments/"+testComment().ID.String(),
		"",
		authHeader(value),
	)
	require.Equal(t, http.StatusOK, res.status)
	require.Equal(t, `{"status":"OK"}`, res.body)
}

// TestDeleteCommentInvalidID covers the identifier check performed before the
// service is reached.
func TestDeleteCommentInvalidID(t *testing.T) {
	h := newTestHarness(t)
	value := h.token()

	res := h.do(http.MethodDelete, "/articles/how-to-train-your-dragon/comments/not-an-id", "", authHeader(value))
	require.Equal(t, http.StatusUnprocessableEntity, res.status)
	require.Equal(t, `{"errors":{"comment_id":["should valid id"]}}`, res.body)
}

// TestArticlePayload pins the article response shape, including the timestamp
// format and the key order.
func TestArticlePayload(t *testing.T) {
	h := newTestHarness(t)

	res := h.get("/articles/how-to-train-your-dragon", nil)
	require.Equal(t, http.StatusOK, res.status)
	require.Equal(
		t,
		`{"article":{"slug":"how-to-train-your-dragon","title":"How to train your dragon",`+
			`"description":"Ever wonder how?","body":"It takes a Jacobian",`+
			`"tagList":["dragons","training"],"createdAt":"2023-09-17T10:30:00Z",`+
			`"updatedAt":"2023-09-17T10:30:00Z","favorited":true,"favoritesCount":2,`+
			`"author":{"username":"alice","bio":"bio","image":"`+domain.UserDefaultImage+`","following":false}}}`,
		res.body,
	)
}

// TestCommentPayload pins the comment response shape.
func TestCommentPayload(t *testing.T) {
	h := newTestHarness(t)
	value := h.token()

	res := h.do(
		http.MethodPost,
		"/articles/how-to-train-your-dragon/comments/",
		`{"comment":{"body":"His name was my name too."}}`,
		authHeader(value),
	)
	require.Equal(t, http.StatusOK, res.status)
	require.Equal(
		t,
		`{"comment":{"id":"01HAJ0000000000000000000CC","createdAt":"2023-09-17T10:30:00Z",`+
			`"updatedAt":"2023-09-17T10:30:00Z","body":"His name was my name too.",`+
			`"author":{"username":"alice","bio":"bio","image":"`+domain.UserDefaultImage+`","following":false}}}`,
		res.body,
	)
}

// TestRecoverMiddleware checks a panicking handler is contained and answered
// with a bare 500.
func TestRecoverMiddleware(t *testing.T) {
	h := newTestHarness(t)
	h.service.article.listTags = func(context.Context) ([]string, error) {
		panic("boom")
	}

	res := h.get("/tags/", nil)
	require.Equal(t, http.StatusInternalServerError, res.status)
	require.Empty(t, res.body)
}
