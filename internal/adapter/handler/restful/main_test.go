package restful

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labasubagia/realworld-backend/internal/adapter/logger"
	"github.com/labasubagia/realworld-backend/internal/core/domain"
	"github.com/labasubagia/realworld-backend/internal/core/port"
	"github.com/labasubagia/realworld-backend/internal/core/util"
	"github.com/labasubagia/realworld-backend/internal/core/util/token"
	"github.com/stretchr/testify/require"
)

const testSymmetricKey = "12345678901234567890123456789012"

// stubUserService satisfies port.UserService. Every method delegates to the
// matching function field so a test can decide what the service layer returns
// without standing up a database.
type stubUserService struct {
	register func(context.Context, port.RegisterParams) (domain.User, error)
	login    func(context.Context, port.LoginParams) (domain.User, error)
	update   func(context.Context, port.UpdateUserParams) (domain.User, error)
	current  func(context.Context, port.AuthParams) (domain.User, error)
	profile  func(context.Context, port.ProfileParams) (domain.User, error)
	follow   func(context.Context, port.ProfileParams) (domain.User, error)
	unfollow func(context.Context, port.ProfileParams) (domain.User, error)
}

func (s *stubUserService) Register(ctx context.Context, arg port.RegisterParams) (domain.User, error) {
	if s.register == nil {
		return testUser(), nil
	}
	return s.register(ctx, arg)
}

func (s *stubUserService) Login(ctx context.Context, arg port.LoginParams) (domain.User, error) {
	if s.login == nil {
		return testUser(), nil
	}
	return s.login(ctx, arg)
}

func (s *stubUserService) Update(ctx context.Context, arg port.UpdateUserParams) (domain.User, error) {
	if s.update == nil {
		return testUser(), nil
	}
	return s.update(ctx, arg)
}

func (s *stubUserService) Current(ctx context.Context, arg port.AuthParams) (domain.User, error) {
	if s.current == nil {
		return testUser(), nil
	}
	return s.current(ctx, arg)
}

func (s *stubUserService) Profile(ctx context.Context, arg port.ProfileParams) (domain.User, error) {
	if s.profile == nil {
		return testUser(), nil
	}
	return s.profile(ctx, arg)
}

func (s *stubUserService) Follow(ctx context.Context, arg port.ProfileParams) (domain.User, error) {
	if s.follow == nil {
		return testUser(), nil
	}
	return s.follow(ctx, arg)
}

func (s *stubUserService) UnFollow(ctx context.Context, arg port.ProfileParams) (domain.User, error) {
	if s.unfollow == nil {
		return testUser(), nil
	}
	return s.unfollow(ctx, arg)
}

// stubArticleService satisfies port.ArticleService with the same delegation
// scheme as stubUserService.
type stubArticleService struct {
	create         func(context.Context, port.CreateArticleTxParams) (domain.Article, error)
	update         func(context.Context, port.UpdateArticleParams) (domain.Article, error)
	del            func(context.Context, port.DeleteArticleParams) error
	list           func(context.Context, port.ListArticleParams) ([]domain.Article, error)
	feed           func(context.Context, port.ListArticleParams) ([]domain.Article, error)
	get            func(context.Context, port.GetArticleParams) (domain.Article, error)
	addComment     func(context.Context, port.AddCommentParams) (domain.Comment, error)
	listComments   func(context.Context, port.ListCommentParams) ([]domain.Comment, error)
	deleteComment  func(context.Context, port.DeleteCommentParams) error
	addFavorite    func(context.Context, port.AddFavoriteParams) (domain.Article, error)
	removeFavorite func(context.Context, port.RemoveFavoriteParams) (domain.Article, error)
	listTags       func(context.Context) ([]string, error)
}

func (s *stubArticleService) Create(ctx context.Context, arg port.CreateArticleTxParams) (domain.Article, error) {
	if s.create == nil {
		return testArticle(), nil
	}
	return s.create(ctx, arg)
}

func (s *stubArticleService) Update(ctx context.Context, arg port.UpdateArticleParams) (domain.Article, error) {
	if s.update == nil {
		return testArticle(), nil
	}
	return s.update(ctx, arg)
}

func (s *stubArticleService) Delete(ctx context.Context, arg port.DeleteArticleParams) error {
	if s.del == nil {
		return nil
	}
	return s.del(ctx, arg)
}

func (s *stubArticleService) List(ctx context.Context, arg port.ListArticleParams) ([]domain.Article, error) {
	if s.list == nil {
		return nil, nil
	}
	return s.list(ctx, arg)
}

func (s *stubArticleService) Feed(ctx context.Context, arg port.ListArticleParams) ([]domain.Article, error) {
	if s.feed == nil {
		return nil, nil
	}
	return s.feed(ctx, arg)
}

func (s *stubArticleService) Get(ctx context.Context, arg port.GetArticleParams) (domain.Article, error) {
	if s.get == nil {
		return testArticle(), nil
	}
	return s.get(ctx, arg)
}

func (s *stubArticleService) AddComment(ctx context.Context, arg port.AddCommentParams) (domain.Comment, error) {
	if s.addComment == nil {
		return testComment(), nil
	}
	return s.addComment(ctx, arg)
}

func (s *stubArticleService) ListComments(ctx context.Context, arg port.ListCommentParams) ([]domain.Comment, error) {
	if s.listComments == nil {
		return nil, nil
	}
	return s.listComments(ctx, arg)
}

func (s *stubArticleService) DeleteComment(ctx context.Context, arg port.DeleteCommentParams) error {
	if s.deleteComment == nil {
		return nil
	}
	return s.deleteComment(ctx, arg)
}

func (s *stubArticleService) AddFavorite(ctx context.Context, arg port.AddFavoriteParams) (domain.Article, error) {
	if s.addFavorite == nil {
		return testArticle(), nil
	}
	return s.addFavorite(ctx, arg)
}

func (s *stubArticleService) RemoveFavorite(ctx context.Context, arg port.RemoveFavoriteParams) (domain.Article, error) {
	if s.removeFavorite == nil {
		return testArticle(), nil
	}
	return s.removeFavorite(ctx, arg)
}

func (s *stubArticleService) ListTags(ctx context.Context) ([]string, error) {
	if s.listTags == nil {
		return nil, nil
	}
	return s.listTags(ctx)
}

// stubService satisfies port.Service. The token maker is the real JWT maker so
// the authorization middleware is exercised end to end.
type stubService struct {
	maker   token.Maker
	user    *stubUserService
	article *stubArticleService
}

func (s *stubService) TokenMaker() token.Maker { return s.maker }
func (s *stubService) User() port.UserService  { return s.user }
func (s *stubService) Article() port.ArticleService {
	return s.article
}

var testTime = time.Date(2023, time.September, 17, 10, 30, 0, 0, time.UTC)

func testUser() domain.User {
	return domain.User{
		ID:        domain.ID("01HAJ0000000000000000000AA"),
		Email:     "alice@realworld.test",
		Username:  "alice",
		Bio:       "bio",
		Image:     domain.UserDefaultImage,
		CreatedAt: testTime,
		UpdatedAt: testTime,
		Token:     "test-token",
	}
}

func testArticle() domain.Article {
	return domain.Article{
		ID:            domain.ID("01HAJ0000000000000000000BB"),
		AuthorID:      testUser().ID,
		Title:         "How to train your dragon",
		Slug:          "how-to-train-your-dragon",
		Description:   "Ever wonder how?",
		Body:          "It takes a Jacobian",
		TagNames:      []string{"dragons", "training"},
		CreatedAt:     testTime,
		UpdatedAt:     testTime,
		Author:        testUser(),
		IsFavorite:    true,
		FavoriteCount: 2,
	}
}

func testComment() domain.Comment {
	return domain.Comment{
		ID:        domain.ID("01HAJ0000000000000000000CC"),
		ArticleID: testArticle().ID,
		AuthorID:  testUser().ID,
		Body:      "His name was my name too.",
		CreatedAt: testTime,
		UpdatedAt: testTime,
		Author:    testUser(),
	}
}

// testHarness owns an httptest server wired to the real router so responses are
// observed exactly as a client sees them: real status line, real header block,
// real body bytes.
type testHarness struct {
	t       *testing.T
	server  *Server
	http    *httptest.Server
	service *stubService
}

func newTestHarness(t *testing.T) *testHarness {
	t.Helper()

	maker, err := token.NewJWTMaker(testSymmetricKey)
	require.NoError(t, err)

	config := util.Config{
		Environment:       util.EnvDevelopment,
		LogType:           logger.TypeZeroLog,
		ServerType:        TypeRestful,
		ServerPort:        0,
		TokenSymmetricKey: testSymmetricKey,
	}
	service := &stubService{
		maker:   maker,
		user:    &stubUserService{},
		article: &stubArticleService{},
	}
	server, ok := NewServer(config, service, logger.NewLogger(config)).(*Server)
	require.True(t, ok)

	httpServer := httptest.NewServer(server.router)
	t.Cleanup(httpServer.Close)

	return &testHarness{t: t, server: server, http: httpServer, service: service}
}

// token mints a signed token for the canned user.
func (h *testHarness) token() string {
	h.t.Helper()
	value, _, err := h.service.maker.CreateToken(testUser().ID, time.Minute)
	require.NoError(h.t, err)
	return value
}

type testResponse struct {
	status int
	header http.Header
	body   string
}

// do issues a request without following redirects so 301/307 responses are
// observable.
func (h *testHarness) do(method, path string, body string, headers map[string]string) testResponse {
	h.t.Helper()

	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, h.http.URL+path, reader)
	require.NoError(h.t, err)
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	client := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	res, err := client.Do(req)
	require.NoError(h.t, err)
	defer res.Body.Close()

	raw := make([]byte, 0, 1024)
	buf := make([]byte, 512)
	for {
		n, readErr := res.Body.Read(buf)
		raw = append(raw, buf[:n]...)
		if readErr != nil {
			break
		}
	}
	return testResponse{status: res.StatusCode, header: res.Header, body: string(raw)}
}

func (h *testHarness) get(path string, headers map[string]string) testResponse {
	return h.do(http.MethodGet, path, "", headers)
}

func authHeader(value string) map[string]string {
	return map[string]string{"Authorization": "Token " + value}
}
