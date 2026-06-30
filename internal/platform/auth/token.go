package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID int      `json:"user_id"`
	Roles  []string `json:"roles"`
	jwt.RegisteredClaims
}

type TokenService struct {
	secret []byte
}

func NewTokenService(secret string) TokenService {
	return TokenService{secret: []byte(secret)}
}

func (s TokenService) IssueAccessToken(userID int, roles []string, ttl time.Duration) (string, time.Time, error) {
	now := time.Now()
	expiresAt := now.Add(ttl)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		UserID: userID,
		Roles:  roles,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.Itoa(userID),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	})
	value, err := token.SignedString(s.secret)
	if err != nil {
		return "", time.Time{}, err
	}
	return value, expiresAt, nil
}

func (s TokenService) ParseAccessToken(value string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(value, &Claims{}, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return s.secret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}

func NewRefreshToken() (string, string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}
	value := base64.RawURLEncoding.EncodeToString(raw)
	return value, HashRefreshToken(value), nil
}

func HashRefreshToken(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
