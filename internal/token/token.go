package token

import (
	"errors"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var ErrInvalid = errors.New("token tidak valid")

func secret() []byte {
	return []byte(os.Getenv("JWT_SECRET"))
}

func Generate(userID uint) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"sub": userID,
		"iat": now.Unix(),
		"exp": now.Add(ExpireDuration).Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret())
}

func Parse(tokenStr string) (uint, error) {
	parsed, err := jwt.Parse(tokenStr, func(*jwt.Token) (interface{}, error) {
		return secret(), nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil || !parsed.Valid {
		return 0, ErrInvalid
	}
	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return 0, ErrInvalid
	}
	sub, ok := claims["sub"].(float64)
	if !ok {
		return 0, ErrInvalid
	}
	return uint(sub), nil
}
