package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/rudraa2005/LogiLens/api-gateway-service/auth"
)

type contextKey string

const userContextKey contextKey = "authenticatedUser"

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "missing authorization header", http.StatusUnauthorized)
			return
		}
		parts := strings.Fields(authHeader)
		if len(parts) != 2 || parts[0] != "Bearer" {
			http.Error(w, "invalid", http.StatusUnauthorized)
			return
		}
		tokenString := parts[1]

		claims := &auth.Claims{}
		parsedClaims, err := auth.ParseJWT(tokenString, claims)
		if err != nil {
			http.Error(w, "invalid or expired", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), userContextKey, parsedClaims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
