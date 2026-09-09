package restful

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/labasubagia/realworld-backend/internal/core/domain"
	"github.com/labasubagia/realworld-backend/internal/core/port"
)

func (s *Server) Logger() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rw := writer(w)

			// request id
			reqID := r.Header.Get("x-request-id")
			if reqID == "" {
				reqID = domain.NewID().String()
			}

			// make logger and sub-logger
			// ? make unique instance for each handler/interactor request
			logger := s.logger.NewInstance().Field("request_id", reqID).Logger()
			// logger := s.logger.Field("request_id", reqID).Logger() // this is use single instance
			r = r.WithContext(context.WithValue(r.Context(), port.SubLoggerCtxKey, logger))

			// process request
			startTime := time.Now()
			next.ServeHTTP(rw, r)
			duration := time.Since(startTime)

			// log
			logEvent := logger.Info()
			if rw.Status() >= 500 {
				logEvent = logger.Error()
				if r.Body != nil {
					if body, err := io.ReadAll(r.Body); err == nil {
						logEvent.Field("body", body)
					}
				}
			}
			logEvent.
				Field("protocol", "http").
				Field("client_ip", clientIP(r)).
				Field("user_agent", r.UserAgent()).
				Field("method", r.Method).
				Field("path", r.URL.Path).
				Field("status_code", rw.Status()).
				Field("status", http.StatusText(rw.Status())).
				Field("duration", duration).
				Msg("received http request")
		})
	}
}
