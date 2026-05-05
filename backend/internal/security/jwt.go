package security

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTManager 负责生成和校验访问令牌。
type JWTManager struct {
	secret []byte
	issuer string
	ttl    time.Duration
}

// NewJWTManager 创建 JWT 管理器。
func NewJWTManager(secret, issuer string, ttl time.Duration) *JWTManager {
	return &JWTManager{
		secret: []byte(secret),
		issuer: issuer,
		ttl:    ttl,
	}
}

// Generate 为指定用户生成 token。
func (m *JWTManager) Generate(userID uint64, username string, email string) (string, time.Time, error) {
	now := time.Now()
	exp := now.Add(m.ttl)
	claims := jwt.MapClaims{
		"sub":      strconv.FormatUint(userID, 10),
		"user_id":  strconv.FormatUint(userID, 10),
		"username": username,
		"email":    email,
		"iss":      m.issuer,
		"iat":      now.Unix(),
		"exp":      exp.Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, err
	}

	return signed, exp, nil
}

// Parse 验证 token 并返回用户 ID。
func (m *JWTManager) Parse(tokenString string) (uint64, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("不支持的签名方式")
		}
		return m.secret, nil
	}, jwt.WithIssuer(m.issuer))
	if err != nil {
		return 0, err
	}

	if !token.Valid {
		return 0, errors.New("token 无效")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return 0, errors.New("token 载荷无效")
	}

	rawID, ok := claims["user_id"].(string)
	if !ok || rawID == "" {
		if sub, subOK := claims["sub"].(string); subOK {
			rawID = sub
		}
	}
	if rawID == "" {
		return 0, errors.New("缺少用户信息")
	}

	userID, err := strconv.ParseUint(rawID, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("用户 ID 解析失败: %w", err)
	}

	return userID, nil
}
