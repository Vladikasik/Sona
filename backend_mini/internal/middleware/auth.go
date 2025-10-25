package middleware

import (
	"net/http"
)

func RequireBearer(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer "+token {
			w.WriteHeader(http.StatusUnauthorized)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}
