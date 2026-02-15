package middleware

import (
	"context"
	"errors"
	"net/http"

	"github.com/rudraa2005/LogiLens/api-gateway-service/auth"
)

var ErrUnauthenticated = errors.New("user not authenticated")

func SetUserInContext(ctx context.Context, claims *auth.Claims) context.Context {
	return context.WithValue(ctx, userContextKey, claims)
}

func GetUser(r *http.Request) (*auth.Claims, error) {
	claims, ok := r.Context().Value(userContextKey).(*auth.Claims)
	if !ok || claims == nil {
		return nil, ErrUnauthenticated
	}
	return claims, nil
}

func GetUserFromContext(ctx context.Context) (*auth.Claims, bool) {
	claims, ok := ctx.Value(userContextKey).(*auth.Claims)
	return claims, ok
}
