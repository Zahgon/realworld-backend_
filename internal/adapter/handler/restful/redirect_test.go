package restful

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestTrailingSlashRedirectHonoursForwardedPrefix pins the header the source
// router reads when it builds a redirect target. Behind a reverse proxy that
// mounts the application under a prefix, a Location that omits the prefix
// points the client at a URL that does not exist. Nothing else in the suite
// sends this header, which is how the omission survived a green run.
func TestTrailingSlashRedirectHonoursForwardedPrefix(t *testing.T) {
	h := newTestHarness(t)

	res := h.do(http.MethodGet, "/tags", "", map[string]string{
		"X-Forwarded-Prefix": "/api",
	})
	require.Equal(t, http.StatusMovedPermanently, res.status)
	require.Equal(t, "/api/tags/", res.header.Get("Location"),
		"the prefix must appear in the Location; without it the client is sent to a path the proxy does not serve")
}

// TestTrailingSlashRedirectDecidesOnTheUnprefixedPath is the other half of the
// same behaviour: the prefix must shape the Location and nothing else. If it
// reached the route lookup, no route would carry it, the redirect would be
// abandoned, and every proxied request would 404.
func TestTrailingSlashRedirectDecidesOnTheUnprefixedPath(t *testing.T) {
	h := newTestHarness(t)

	withPrefix := h.do(http.MethodGet, "/tags", "", map[string]string{
		"X-Forwarded-Prefix": "/api",
	})
	without := h.do(http.MethodGet, "/tags", "", nil)

	require.Equal(t, without.status, withPrefix.status,
		"the header must not change whether a redirect happens, only where it points")
	require.Equal(t, http.StatusMovedPermanently, without.status)
}

// TestForwardedPrefixIsSanitised covers the two substitutions the source router
// applies before using the header: characters outside [a-zA-Z0-9/-] are dropped
// and runs of slashes are collapsed. A prefix is attacker-controlled input, so
// echoing it unfiltered into a Location header is a header-injection vector.
func TestForwardedPrefixIsSanitised(t *testing.T) {
	h := newTestHarness(t)

	testCases := []struct {
		name   string
		prefix string
		want   string
	}{
		{name: "unsafe characters dropped", prefix: "/ap<i>", want: "/api/tags/"},
		{name: "repeated slashes cleaned", prefix: "//api///v1", want: "/api/v1/tags/"},
		{name: "dot segments cleaned", prefix: "/api/../api", want: "/api/tags/"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			res := h.do(http.MethodGet, "/tags", "", map[string]string{
				"X-Forwarded-Prefix": tc.prefix,
			})
			require.Equal(t, http.StatusMovedPermanently, res.status)
			require.Equal(t, tc.want, res.header.Get("Location"))
		})
	}
}

// TestTrailingSlashRedirectPreservesQueryString guards the redirect against
// being rewritten to use the path alone. The source router redirects to the
// full URL, so a client that follows the Location keeps its query parameters;
// dropping them silently discards pagination and filters.
func TestTrailingSlashRedirectPreservesQueryString(t *testing.T) {
	h := newTestHarness(t)

	res := h.do(http.MethodGet, "/articles?limit=2&offset=4", "", nil)
	require.Equal(t, http.StatusMovedPermanently, res.status)
	require.Equal(t, "/articles/?limit=2&offset=4", res.header.Get("Location"))
}

// TestEmptyPathSegmentsDispatchLikeTheSourceRouter pins how empty path
// segments are routed. The answer is not uniform, and it was measured against
// the running source server rather than assumed:
//
//	DELETE /articles/            -> 404 {"message":"page not found"}            (28 bytes, router)
//	DELETE /articles//favorite/  -> 404 {"errors":{"exception":["article not found"]}} (46 bytes, handler)
//	GET    /articles//comments/  -> 404 {"errors":{"exception":["article not found"]}} (46 bytes, handler)
//
// The differing bodies are the evidence. A parameter as the *final* segment
// refuses to match an empty string, so the request stops at the router; a
// parameter in the *middle* of a path matches with an empty value and
// dispatches into the handler, which then fails its own lookup. Pinning both
// halves stops a tidier-looking empty-segment guard from being added later,
// which would silently convert those handler answers into router 404s.
func TestEmptyPathSegmentsDispatchLikeTheSourceRouter(t *testing.T) {
	h := newTestHarness(t)

	// DELETE "/articles/{slug}" is the discriminating shape for the final
	// segment: "/articles/" has the same segment count and would match were an
	// empty parameter accepted. GET is no good here - "/articles/" is a
	// registered literal route for GET, so it matches either way.
	require.False(t, h.server.router.match(http.MethodDelete, "/articles/"),
		"an empty parameter as the final segment must not count as a registered route")
	res := h.do(http.MethodDelete, "/articles/", "", nil)
	require.Equal(t, http.StatusNotFound, res.status)
	require.JSONEq(t, `{"message":"page not found"}`, res.body)

	// A mid-path empty parameter matches and reaches the handler. Against the
	// stub service the article lookup succeeds, so the observable proof that
	// dispatch happened is an article payload where the router-level answer
	// would have been the "page not found" body above.
	require.True(t, h.server.router.match(http.MethodDelete, "/articles//favorite/"),
		"an empty parameter in the middle of a path matches, as it does in the source")
	res = h.do(http.MethodDelete, "/articles//favorite/", "", authHeader(h.token()))
	require.Equal(t, http.StatusOK, res.status)
	require.Contains(t, res.body, `"article"`)

	require.True(t, h.server.router.match(http.MethodDelete, "/articles/some-slug"),
		"a populated parameter segment still matches")
}

// TestCORSPreflightOmitsAllowCredentials pins the preflight header set. The
// source router advertises a wildcard origin, and a wildcard origin combined
// with Access-Control-Allow-Credentials is rejected by every browser - adding
// the header would break exactly the cross-origin requests CORS exists to
// permit, while every same-origin test kept passing.
func TestCORSPreflightOmitsAllowCredentials(t *testing.T) {
	h := newTestHarness(t)

	res := h.do(http.MethodOptions, "/articles/", "", map[string]string{
		"Origin":                        "http://example.com",
		"Access-Control-Request-Method": "POST",
	})
	require.Equal(t, "*", res.header.Get("Access-Control-Allow-Origin"))
	require.Empty(t, res.header.Get("Access-Control-Allow-Credentials"),
		"a wildcard origin must never be paired with credentials")
}
