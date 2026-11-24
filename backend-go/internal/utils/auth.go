package utils

import (
    "crypto/rand"
    "encoding/base64"
    "errors"
    "time"

    "github.com/golang-jwt/jwt/v5"
    "golang.org/x/crypto/bcrypt"
)

type Claims struct {
    UserID     int64  `json:"user_id"`
    SystemRole string `json:"system_role"`
    jwt.RegisteredClaims
}

func HashPassword(password string) (string, error) {
    hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
    return string(hashed), err
}

func CheckPassword(hash, password string) bool {
    if hash == "" {
        return false
    }
    return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func GenerateJWT(userID int64, role, secret string) (string, error) {
    claims := Claims{
        UserID:     userID,
        SystemRole: role,
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
        },
    }
    return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

func ParseJWT(tokenStr, secret string) (*Claims, error) {
    token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
        return []byte(secret), nil
    })
    if err != nil || !token.Valid {
        return nil, errors.New("token không hợp lệ hoặc đã hết hạn")
    }
    claims, ok := token.Claims.(*Claims)
    if !ok {
        return nil, errors.New("không thể đọc thông tin token")
    }
    return claims, nil
}

func RandomState() string {
    b := make([]byte, 24)
    _, _ = rand.Read(b)
    return base64.URLEncoding.EncodeToString(b)
}