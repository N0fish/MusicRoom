package main

import (
    "time"

    "github.com/golang-jwt/jwt/v5"
)

type TokenClaims struct {
    UserID        string `json:"uid"`
    Email         string `json:"email"`
    EmailVerified bool   `json:"emailVerified"`
    TokenType     string `json:"typ"`
    jwt.RegisteredClaims
}

type AuthTokens struct {
    AccessToken  string `json:"accessToken"`
    RefreshToken string `json:"refreshToken"`
}

func (s *Server) issueTokens(user AuthUser) (AuthTokens, error) {
    now := time.Now()

    accessClaims := &TokenClaims{
        UserID:        user.ID,
        Email:         user.Email,
        EmailVerified: user.EmailVerified,
        TokenType:     "access",
        RegisteredClaims: jwt.RegisteredClaims{
            Subject:   user.ID,
            IssuedAt:  jwt.NewNumericDate(now),
            ExpiresAt: jwt.NewNumericDate(now.Add(s.accessTTL)),
        },
    }
    accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
    accessStr, err := accessToken.SignedString(s.jwtSecret)
    if err != nil {
        return AuthTokens{}, err
    }

    refreshClaims := &TokenClaims{
        UserID:        user.ID,
        Email:         user.Email,
        EmailVerified: user.EmailVerified,
        TokenType:     "refresh",
        RegisteredClaims: jwt.RegisteredClaims{
            Subject:   user.ID,
            IssuedAt:  jwt.NewNumericDate(now),
            ExpiresAt: jwt.NewNumericDate(now.Add(s.refreshTTL)),
        },
    }
    refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
    refreshStr, err := refreshToken.SignedString(s.jwtSecret)
    if err != nil {
        return AuthTokens{}, err
    }

    return AuthTokens{
        AccessToken:  accessStr,
        RefreshToken: refreshStr,
    }, nil
}
