package restful

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/labasubagia/realworld-backend/internal/core/domain"
	"github.com/labasubagia/realworld-backend/internal/core/port"
	"github.com/labasubagia/realworld-backend/internal/core/util/exception"
)

func (server *Server) ListArticle(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	Tag := query.Get("tag")
	Author := query.Get("author")
	FavoritedBy := query.Get("favorited")

	offset, limit := getPagination(r)
	authArg, _ := getAuthArg(r)

	arg := port.ListArticleParams{
		Tags:           []string{},
		AuthorNames:    []string{},
		FavoritedNames: []string{},
		AuthArg:        authArg,
		Offset:         offset,
		Limit:          limit,
	}
	if Tag != "" {
		arg.Tags = append(arg.Tags, Tag)
	}
	if Author != "" {
		arg.AuthorNames = append(arg.AuthorNames, Author)
	}
	if FavoritedBy != "" {
		arg.FavoritedNames = append(arg.FavoritedNames, FavoritedBy)
	}

	articles, err := server.service.Article().List(r.Context(), arg)
	if err != nil {
		errorHandler(w, err)
		return
	}

	res := ArticlesResponse{
		Articles: []Article{},
		Count:    len(articles),
	}
	for _, article := range articles {
		res.Articles = append(res.Articles, serializeArticle(article))
	}

	writeJSON(w, http.StatusOK, res)
}

func (server *Server) FeedArticle(w http.ResponseWriter, r *http.Request) {
	offset, limit := getPagination(r)
	authArg, err := getAuthArg(r)
	if err != nil {
		errorHandler(w, err)
		return
	}

	arg := port.ListArticleParams{
		AuthArg: authArg,
		Offset:  offset,
		Limit:   limit,
	}
	articles, err := server.service.Article().Feed(r.Context(), arg)
	if err != nil {
		errorHandler(w, err)
		return
	}

	res := ArticlesResponse{
		Articles: []Article{},
		Count:    len(articles),
	}
	for _, article := range articles {
		res.Articles = append(res.Articles, serializeArticle(article))
	}

	writeJSON(w, http.StatusOK, res)
}

func (server *Server) GetArticle(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	authArg, _ := getAuthArg(r)

	article, err := server.service.Article().Get(r.Context(), port.GetArticleParams{
		AuthArg: authArg,
		Slug:    slug,
	})
	if err != nil {
		errorHandler(w, err)
		return
	}

	res := ArticleResponse{serializeArticle(article)}
	writeJSON(w, http.StatusOK, res)
}

type CreateArticle struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Body        string   `json:"body"`
	TagList     []string `json:"tagList"`
}

type CreateArticleRequest struct {
	Article CreateArticle `json:"article"`
}

func (server *Server) CreateArticle(w http.ResponseWriter, r *http.Request) {
	authArg, err := getAuthArg(r)
	if err != nil {
		errorHandler(w, err)
		return
	}

	var req CreateArticleRequest
	if err := bindJSON(w, r, &req); err != nil {
		errorHandler(w, err)
		return
	}

	article, err := server.service.Article().Create(r.Context(), port.CreateArticleTxParams{
		AuthArg: authArg,
		Tags:    req.Article.TagList,
		Article: domain.Article{
			Title:       req.Article.Title,
			Description: req.Article.Description,
			Body:        req.Article.Body,
		},
	})
	if err != nil {
		errorHandler(w, err)
		return
	}

	res := ArticleResponse{serializeArticle(article)}
	writeJSON(w, http.StatusCreated, res)
}

type UpdateArticle struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Body        string `json:"body"`
}

type UpdateArticleRequest struct {
	Article UpdateArticle `json:"article"`
}

func (server *Server) UpdateArticle(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	authArg, err := getAuthArg(r)
	if err != nil {
		errorHandler(w, err)
		return
	}

	var req UpdateArticleRequest
	if err := bindJSON(w, r, &req); err != nil {
		errorHandler(w, err)
		return
	}

	article, err := server.service.Article().Update(r.Context(), port.UpdateArticleParams{
		AuthArg: authArg,
		Slug:    slug,
		Article: domain.Article{
			Title:       req.Article.Title,
			Description: req.Article.Description,
			Body:        req.Article.Body,
		},
	})
	if err != nil {
		errorHandler(w, err)
		return
	}

	res := ArticleResponse{serializeArticle(article)}
	writeJSON(w, http.StatusOK, res)
}

func (server *Server) DeleteArticle(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	authArg, err := getAuthArg(r)
	if err != nil {
		errorHandler(w, err)
		return
	}

	err = server.service.Article().Delete(r.Context(), port.DeleteArticleParams{
		AuthArg: authArg,
		Slug:    slug,
	})
	if err != nil {
		errorHandler(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"status": "OK"})
}

type AddCommentRequest struct {
	Comment Comment `json:"comment"`
}

func (server *Server) AddComment(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	authArg, err := getAuthArg(r)
	if err != nil {
		errorHandler(w, err)
		return
	}

	var req AddCommentRequest
	if err := bindJSON(w, r, &req); err != nil {
		errorHandler(w, err)
		return
	}

	result, err := server.service.Article().AddComment(r.Context(), port.AddCommentParams{
		AuthArg: authArg,
		Slug:    slug,
		Comment: domain.Comment{
			Body: req.Comment.Body,
		},
	})
	if err != nil {
		errorHandler(w, err)
		return
	}

	res := CommentResponse{serializeComment(result)}
	writeJSON(w, http.StatusOK, res)
}

func (server *Server) ListComments(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	authArg, _ := getAuthArg(r)

	comments, err := server.service.Article().ListComments(r.Context(), port.ListCommentParams{
		AuthArg: authArg,
		Slug:    slug,
	})
	if err != nil {
		errorHandler(w, err)
		return
	}

	res := CommentsResponse{
		Comments: []Comment{},
	}
	for _, comment := range comments {
		res.Comments = append(res.Comments, serializeComment(comment))
	}
	writeJSON(w, http.StatusOK, res)
}

func (server *Server) DeleteComment(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	commentID, err := domain.ParseID(chi.URLParam(r, "comment_id"))
	if err != nil {
		err = exception.Validation().AddError("comment_id", "should valid id")
		errorHandler(w, err)
		return
	}

	authArg, err := getAuthArg(r)
	if err != nil {
		errorHandler(w, err)
		return
	}

	err = server.service.Article().DeleteComment(r.Context(), port.DeleteCommentParams{
		AuthArg:   authArg,
		Slug:      slug,
		CommentID: domain.ID(commentID),
	})
	if err != nil {
		errorHandler(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"status": "OK"})
}

func (server *Server) AddFavoriteArticle(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	authArg, err := getAuthArg(r)
	if err != nil {
		errorHandler(w, err)
		return
	}

	article, err := server.service.Article().AddFavorite(r.Context(), port.AddFavoriteParams{
		AuthArg: authArg,
		Slug:    slug,
		UserID:  authArg.Payload.UserID,
	})
	if err != nil {
		errorHandler(w, err)
		return
	}

	res := ArticleResponse{
		Article: serializeArticle(article),
	}

	writeJSON(w, http.StatusOK, res)
}

func (server *Server) RemoveFavoriteArticle(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	authArg, err := getAuthArg(r)
	if err != nil {
		errorHandler(w, err)
		return
	}

	article, err := server.service.Article().RemoveFavorite(r.Context(), port.RemoveFavoriteParams{
		AuthArg: authArg,
		Slug:    slug,
		UserID:  authArg.Payload.UserID,
	})
	if err != nil {
		errorHandler(w, err)
		return
	}

	res := ArticleResponse{serializeArticle(article)}
	writeJSON(w, http.StatusOK, res)
}

func (server *Server) ListTags(w http.ResponseWriter, r *http.Request) {
	tags, err := server.service.Article().ListTags(r.Context())
	if err != nil {
		errorHandler(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tags": tags})
}
