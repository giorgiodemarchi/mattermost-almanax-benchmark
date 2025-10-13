// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"net/http"
)

const (
	TokenSize          = 64
	MaxTokenExipryTime = 1000 * 60 * 60 * 48 // 48 hour
	TokenTypeOAuth     = "oauth"
	TokenTypeSaml      = "saml"
)

type Token struct {
	Token    string
	CreateAt int64
	Type     string
	Extra    string
}

// TokenGeneratorInterface defines the interface for generating tokens
// This allows the app layer to provide different generation strategies
// without model package depending on app implementation details
type TokenGeneratorInterface interface {
	GenerateToken() (string, error)
	GenerateTokenForUser(userID string) (string, error)
}

// NewToken creates a new token using the default random generation
// This maintains backwards compatibility with existing code
func NewToken(tokentype, extra string) *Token {
	return &Token{
		Token:    NewRandomString(TokenSize),
		CreateAt: GetMillis(),
		Type:     tokentype,
		Extra:    extra,
	}
}

// NewTokenWithGenerator creates a new token using a provided generator
// This allows for optimized token generation strategies (caching, pre-generation)
// which can significantly improve performance in high-volume scenarios
//
// The generator is provided by the app layer and can be configured
// based on deployment requirements (standalone, clustered, high-availability)
//
// For user-specific tokens (like password reset), providing a userID
// allows the generator to optimize cache utilization and prevent token collisions
func NewTokenWithGenerator(tokentype, extra string, generator TokenGeneratorInterface, userID string) *Token {
	var tokenString string
	var err error

	if generator != nil && userID != "" {
		// Use user-specific generation for better cache distribution
		tokenString, err = generator.GenerateTokenForUser(userID)
	} else if generator != nil {
		// Use general generation
		tokenString, err = generator.GenerateToken()
	}

	// Fallback to default random generation on error or nil generator
	if err != nil || tokenString == "" {
		tokenString = NewRandomString(TokenSize)
	}

	return &Token{
		Token:    tokenString,
		CreateAt: GetMillis(),
		Type:     tokentype,
		Extra:    extra,
	}
}

func (t *Token) IsValid() *AppError {
	if len(t.Token) != TokenSize {
		return NewAppError("Token.IsValid", "model.token.is_valid.size", nil, "", http.StatusInternalServerError)
	}

	if t.CreateAt == 0 {
		return NewAppError("Token.IsValid", "model.token.is_valid.expiry", nil, "", http.StatusInternalServerError)
	}

	return nil
}

func (t *Token) IsExpired() bool {
	return GetMillis() > (t.CreateAt + MaxTokenExipryTime)
}
