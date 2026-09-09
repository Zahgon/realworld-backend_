package restful

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/labasubagia/realworld-backend/internal/core/domain"
	"github.com/labasubagia/realworld-backend/internal/core/port"
)

type RegisterRequestUser struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type RegisterRequest struct {
	User RegisterRequestUser `json:"user"`
}

func (server *Server) Register(w http.ResponseWriter, r *http.Request) {
	req := RegisterRequest{}
	if err := bindJSON(w, r, &req); err != nil {
		errorHandler(w, err)
		return
	}

	user, err := server.service.User().Register(r.Context(), port.RegisterParams{
		User: domain.User{
			Email:    req.User.Email,
			Username: req.User.Username,
			Password: req.User.Password,
		},
	})
	if err != nil {
		errorHandler(w, err)
		return
	}

	res := UserResponse{serializeUser(user)}
	writeJSON(w, http.StatusCreated, res)
}

type LoginParamUser struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginRequest struct {
	User LoginParamUser `json:"user"`
}

func (server *Server) Login(w http.ResponseWriter, r *http.Request) {
	req := LoginRequest{}
	if err := bindJSON(w, r, &req); err != nil {
		errorHandler(w, err)
		return
	}

	user, err := server.service.User().Login(r.Context(), port.LoginParams{
		User: domain.User{
			Email:    req.User.Email,
			Password: req.User.Password,
		},
	})
	if err != nil {
		errorHandler(w, err)
		return
	}

	res := UserResponse{serializeUser(user)}
	writeJSON(w, http.StatusOK, res)
}

func (server *Server) CurrentUser(w http.ResponseWriter, r *http.Request) {
	authArg, err := getAuthArg(r)
	if err != nil {
		errorHandler(w, err)
		return
	}
	user, err := server.service.User().Current(r.Context(), authArg)
	if err != nil {
		errorHandler(w, err)
		return
	}
	res := UserResponse{serializeUser(user)}
	writeJSON(w, http.StatusOK, res)
}

type UpdateUser struct {
	Email    string `json:"email,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Bio      string `json:"bio,omitempty"`
	Image    string `json:"image,omitempty"`
}

type UpdateUserRequest struct {
	User UpdateUser `json:"user"`
}

func (server *Server) UpdateUser(w http.ResponseWriter, r *http.Request) {
	authArg, err := getAuthArg(r)
	if err != nil {
		errorHandler(w, err)
		return
	}
	req := UpdateUserRequest{}
	if err := bindJSON(w, r, &req); err != nil {
		errorHandler(w, err)
		return
	}
	user, err := server.service.User().Update(r.Context(), port.UpdateUserParams{
		AuthArg: authArg,
		User: domain.User{
			ID:       authArg.Payload.UserID,
			Email:    req.User.Email,
			Username: req.User.Username,
			Password: req.User.Password,
			Image:    req.User.Image,
			Bio:      req.User.Bio,
		},
	})
	if err != nil {
		errorHandler(w, err)
		return
	}
	res := UserResponse{serializeUser(user)}
	writeJSON(w, http.StatusOK, res)
}

func (server *Server) Profile(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")
	authArg, _ := getAuthArg(r)
	user, err := server.service.User().Profile(r.Context(), port.ProfileParams{
		Username: username,
		AuthArg:  authArg,
	})
	if err != nil {
		errorHandler(w, err)
		return
	}
	res := ProfileResponse{serializeProfile(user)}
	writeJSON(w, http.StatusOK, res)
}

func (server *Server) FollowUser(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")
	authArg, err := getAuthArg(r)
	if err != nil {
		errorHandler(w, err)
		return
	}
	user, err := server.service.User().Follow(r.Context(), port.ProfileParams{
		Username: username,
		AuthArg:  authArg,
	})
	if err != nil {
		errorHandler(w, err)
		return
	}
	res := ProfileResponse{serializeProfile(user)}
	writeJSON(w, http.StatusOK, res)
}

func (server *Server) UnFollowUser(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")
	authArg, err := getAuthArg(r)
	if err != nil {
		errorHandler(w, err)
		return
	}
	user, err := server.service.User().UnFollow(r.Context(), port.ProfileParams{
		Username: username,
		AuthArg:  authArg,
	})
	if err != nil {
		errorHandler(w, err)
		return
	}
	res := ProfileResponse{serializeProfile(user)}
	writeJSON(w, http.StatusOK, res)
}
