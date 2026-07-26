package auth

import (
	"errors"
	"fmt"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	jwt.RegisteredClaims
	UserID    string `json:"user_id"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	TokenType string `json:"token_type"`
}
type Auth interface {
	VerifyAccessToken(tokenStr string) (*Claims, error)
}

type auth struct {
	jwtSecret string
	jwtIssuer string
}


func NewAuth(jwtSecret string , jwtIssuer string) Auth {
	return &auth{
		jwtSecret: jwtSecret,
		jwtIssuer: jwtIssuer,
	}
}


// VerifyAccessToken implements [Auth].
func (a *auth) VerifyAccessToken(tokenStr string) (*Claims, error) {
	if tokenStr == "" {
		return nil, errors.New("missing token")
	}

	parser := jwt.NewParser(
		jwt.WithIssuer(a.jwtIssuer),
		jwt.WithAudience(a.jwtIssuer),
	)

	token, err := parser.ParseWithClaims(
		tokenStr,
		&Claims{},
		func(t *jwt.Token) (any, error) {

			if t.Method != jwt.SigningMethodHS256 {
				return nil, fmt.Errorf(
					"unexpected signing method: %v",
					t.Header["alg"],
				)
			}

			return []byte(a.jwtSecret), nil
		},
	)

	if err != nil {
		return nil, fmt.Errorf(
			"invalid access token: %w",
			err,
		)
	}

	claims, ok := token.Claims.(*Claims)

	if !ok || !token.Valid {
		return nil, errors.New("invalid claims")
	}

	if claims.TokenType != "access" {
		return nil, errors.New("invalid token type")
	}

	if claims.UserID == "" {
		return nil, errors.New("missing user id")
	}

	return claims, nil
}