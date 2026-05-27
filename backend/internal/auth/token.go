package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID    string `json:"userId"`
	Role      string `json:"role"`
	TokenType string `json:"tokenType,omitempty"`
	jwt.RegisteredClaims
}

type TokenPair struct {
	Token        string
	RefreshToken string
}

const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"

	accessTokenTTL  = 2 * time.Hour
	refreshTokenTTL = 7 * 24 * time.Hour
)

func IssueToken(secret string, userID string, role string) (string, error) {
	return issueTokenWithClock(secret, userID, role, TokenTypeAccess, 24*time.Hour, time.Now)
}

func IssueTokenPair(secret string, userID string, role string) (TokenPair, error) {
	return IssueTokenPairWithClock(secret, userID, role, time.Now)
}

func IssueTokenPairWithClock(secret string, userID string, role string, now func() time.Time) (TokenPair, error) {
	accessToken, err := issueTokenWithClock(secret, userID, role, TokenTypeAccess, accessTokenTTL, now)
	if err != nil {
		return TokenPair{}, err
	}
	refreshToken, err := issueTokenWithClock(secret, userID, role, TokenTypeRefresh, refreshTokenTTL, now)
	if err != nil {
		return TokenPair{}, err
	}
	return TokenPair{Token: accessToken, RefreshToken: refreshToken}, nil
}

func issueTokenWithClock(secret string, userID string, role string, tokenType string, ttl time.Duration, now func() time.Time) (string, error) {
	if secret == "" {
		return "", errors.New("JWT secret is required")
	}
	issuedAt := now()
	claims := Claims{
		UserID:    userID,
		Role:      role,
		TokenType: tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(issuedAt),
			ExpiresAt: jwt.NewNumericDate(issuedAt.Add(ttl)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func ParseAccessToken(secret string, raw string) (Claims, error) {
	claims, err := ParseToken(secret, raw)
	if err != nil {
		return Claims{}, err
	}
	if claims.TokenType != "" && claims.TokenType != TokenTypeAccess {
		return Claims{}, errors.New("token is not an access token")
	}
	return claims, nil
}

func ParseRefreshToken(secret string, raw string) (Claims, error) {
	claims, err := ParseToken(secret, raw)
	if err != nil {
		return Claims{}, err
	}
	if claims.TokenType != TokenTypeRefresh {
		return Claims{}, errors.New("token is not a refresh token")
	}
	return claims, nil
}

func ParseToken(secret string, raw string) (Claims, error) {
	if secret == "" {
		return Claims{}, errors.New("JWT secret is required")
	}

	token, err := jwt.ParseWithClaims(raw, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil {
		return Claims{}, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return Claims{}, errors.New("invalid token")
	}

	return *claims, nil
}
