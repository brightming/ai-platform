package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Authenticator 认证器接口
type Authenticator interface {
	Authenticate(ctx context.Context, token string) (*AuthInfo, error)
	GenerateToken(info *AuthInfo) (string, error)
	ValidateToken(token string) (*AuthInfo, error)
}

// AuthInfo 认证信息
type AuthInfo struct {
	TenantID string   `json:"tenant_id"`
	UserID   string   `json:"user_id"`
	Roles    []string `json:"roles"`
	Exp      int64    `json:"exp"`
}

// JWTAuth JWT认证器
type JWTAuth struct {
	secret      []byte
	expire      time.Duration
	algo        jwt.SigningMethod
}

// NewJWTAuth 创建JWT认证器
func NewJWTAuth(secret string, expire time.Duration) *JWTAuth {
	return &JWTAuth{
		secret: []byte(secret),
		expire: expire,
		algo:   jwt.SigningMethodHS256,
	}
}

// Claims JWT声明
type Claims struct {
	TenantID string   `json:"tenant_id"`
	UserID   string   `json:"user_id"`
	Roles    []string `json:"roles"`
	jwt.RegisteredClaims
}

// Authenticate 认证
func (a *JWTAuth) Authenticate(ctx context.Context, token string) (*AuthInfo, error) {
	if token == "" {
		return nil, errors.New("empty token")
	}

	// 移除Bearer前缀
	if len(token) > 7 && token[:7] == "Bearer " {
		token = token[7:]
	}

	return a.ValidateToken(token)
}

// GenerateToken 生成Token
func (a *JWTAuth) GenerateToken(info *AuthInfo) (string, error) {
	now := time.Now()
	claims := &Claims{
		TenantID: info.TenantID,
		UserID:   info.UserID,
		Roles:    info.Roles,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(a.expire)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "ai-platform",
		},
	}

	token := jwt.NewWithClaims(a.algo, claims)
	return token.SignedString(a.secret)
}

// ValidateToken 验证Token
func (a *JWTAuth) ValidateToken(tokenString string) (*AuthInfo, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if token.Method != a.algo {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return a.secret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return &AuthInfo{
			TenantID: claims.TenantID,
			UserID:   claims.UserID,
			Roles:    claims.Roles,
			Exp:      claims.ExpiresAt.Unix(),
		}, nil
	}

	return nil, errors.New("invalid token")
}

// APIKeyAuth API密钥认证器
type APIKeyAuth struct {
	keys map[string]*AuthInfo
}

// NewAPIKeyAuth 创建API密钥认证器
func NewAPIKeyAuth() *APIKeyAuth {
	return &APIKeyAuth{
		keys: make(map[string]*AuthInfo),
	}
}

// AddKey 添加密钥
func (a *APIKeyAuth) AddKey(key string, info *AuthInfo) {
	a.keys[key] = info
}

// Authenticate 认证
func (a *APIKeyAuth) Authenticate(ctx context.Context, token string) (*AuthInfo, error) {
	if token == "" {
		return nil, errors.New("empty api key")
	}

	info, ok := a.keys[token]
	if !ok {
		return nil, errors.New("invalid api key")
	}

	return info, nil
}

// GenerateToken 不支持
func (a *APIKeyAuth) GenerateToken(info *AuthInfo) (string, error) {
	return "", errors.New("not supported")
}

// ValidateToken 验证Token
func (a *APIKeyAuth) ValidateToken(token string) (*AuthInfo, error) {
	info, ok := a.keys[token]
	if !ok {
		return nil, errors.New("invalid api key")
	}
	return info, nil
}

// MultiAuth 组合认证器
type MultiAuth struct {
	authenticators []Authenticator
}

// NewMultiAuth 创建组合认证器
func NewMultiAuth(auths ...Authenticator) *MultiAuth {
	return &MultiAuth{authenticators: auths}
}

// Authenticate 依次尝试各认证器
func (m *MultiAuth) Authenticate(ctx context.Context, token string) (*AuthInfo, error) {
	for _, auth := range m.authenticators {
		info, err := auth.Authenticate(ctx, token)
		if err == nil {
			return info, nil
		}
	}
	return nil, errors.New("authentication failed")
}

// GenerateToken 使用第一个认证器生成Token
func (m *MultiAuth) GenerateToken(info *AuthInfo) (string, error) {
	if len(m.authenticators) == 0 {
		return "", errors.New("no authenticator")
	}
	return m.authenticators[0].GenerateToken(info)
}

// ValidateToken 依次尝试各认证器验证Token
func (m *MultiAuth) ValidateToken(token string) (*AuthInfo, error) {
	for _, auth := range m.authenticators {
		info, err := auth.ValidateToken(token)
		if err == nil {
			return info, nil
		}
	}
	return nil, errors.New("invalid token")
}
