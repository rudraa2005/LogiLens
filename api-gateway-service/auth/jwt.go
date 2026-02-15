package auth

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	Email  string `json:"email"`
	UserID string `json:"user_id"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

func CreateToken(userID string, email string, role string) (string, error) {
	claims := Claims{
		userID,
		email,
		role,
		jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   userID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte("jwt-secret-key"))
}
func KeyFunction(token *jwt.Token) (interface{}, error) {
	if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
		return nil, jwt.ErrTokenSignatureInvalid
	}
	return []byte("jwt-secret-key"), nil
}

func ParseJWT(token string, c *Claims) (*Claims, error) {
	parse, err := jwt.ParseWithClaims(token, c, KeyFunction)

	if err != nil {
		return nil, err
	}
	if !parse.Valid {
		return nil, jwt.ErrSignatureInvalid
	}

	return c, err
}
