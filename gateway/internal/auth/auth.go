package auth

import (
	"net/http"
	"strings"
	"sync"

	"github.com/golang-jwt/jwt/v5"
)

var (
	jwtKey     []byte
	jwtKeyOnce sync.Once
)

func getJWTKey() []byte {
	jwtKeyOnce.Do(func() {
		jwtKey = []byte("C4lv0kkk")
	})
	return jwtKey
}

// Middleware checks for valid JWT in Authorization header.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenStr := extractToken(r)
		if tokenStr == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		claims := &jwt.RegisteredClaims{}
		_, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
			return getJWTKey(), nil
		})
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func extractToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if h == "" || !strings.HasPrefix(h, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(h, "Bearer ")
}
