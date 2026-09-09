package restful

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/labasubagia/realworld-backend/internal/core/port"
	"github.com/labasubagia/realworld-backend/internal/core/util"
)

const TypeRestful = "restful"

type Server struct {
	config  util.Config
	router  *router
	service port.Service
	logger  port.Logger
}

func NewServer(config util.Config, service port.Service, logger port.Logger) port.Server {
	server := &Server{
		config:  config,
		service: service,
		logger:  logger,
	}
	server.setupRouter()
	return server
}

func (server *Server) setupRouter() {

	mux := chi.NewRouter()
	rt := newRouter(mux)

	mux.Use(server.Logger(), middleware.Recoverer, cors())

	mux.NotFound(rt.notFound)
	mux.MethodNotAllowed(rt.methodNotAllowed)

	mux.Get("/", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"message": "Hello World!"})
	})
	mux.Post("/users", server.Register)
	mux.Post("/users/login", server.Login)

	mux.Group(func(userRouter chi.Router) {
		userRouter.Use(server.AuthMiddleware(true))
		userRouter.Get("/user/", server.CurrentUser)
		userRouter.Put("/user/", server.UpdateUser)
	})

	mux.Group(func(profileRouter chi.Router) {
		profileRouter.Use(server.AuthMiddleware(false))
		profileRouter.Get("/profiles/{username}/", server.Profile)
		profileRouter.Post("/profiles/{username}/follow", server.FollowUser)
		profileRouter.Delete("/profiles/{username}/follow", server.UnFollowUser)
	})

	mux.Group(func(articleRouter chi.Router) {
		articleRouter.Use(server.AuthMiddleware(false))
		articleRouter.Get("/articles/", server.ListArticle)
		articleRouter.Get("/articles/feed", server.FeedArticle)
		articleRouter.Get("/articles/{slug}", server.GetArticle)
		articleRouter.Post("/articles/", server.CreateArticle)
		articleRouter.Put("/articles/{slug}", server.UpdateArticle)
		articleRouter.Delete("/articles/{slug}", server.DeleteArticle)

		articleRouter.Post("/articles/{slug}/comments/", server.AddComment)
		articleRouter.Get("/articles/{slug}/comments/", server.ListComments)
		articleRouter.Delete("/articles/{slug}/comments/{comment_id}", server.DeleteComment)

		articleRouter.Post("/articles/{slug}/favorite/", server.AddFavoriteArticle)
		articleRouter.Delete("/articles/{slug}/favorite/", server.RemoveFavoriteArticle)
	})

	mux.Get("/tags/", server.ListTags)

	rt.index()
	server.router = rt
}

func (server *Server) Start() error {
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", server.config.ServerPort),
		Handler: server.router,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			server.logger.Fatal().Err(err).Msg("failed listen")
		}
	}()

	quit := make(chan os.Signal)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	server.logger.Info().Msg("shutdown server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		server.logger.Fatal().Err(err).Msg("failed shutdown server")
	}

	select {
	case <-ctx.Done():
		server.logger.Info().Msg("timeout in 5 seconds")
	}
	server.logger.Info().Msg("server exiting")

	return nil
}
