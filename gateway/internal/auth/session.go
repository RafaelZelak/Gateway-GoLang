package auth

import (
	"html/template"
	"log"
	"net/http"
	"path"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var jwtKey = []byte("C4lv0kkk")

type Claims struct {
	Username string `json:"username"`
	Scope    string `json:"scope"`
	jwt.RegisteredClaims
}

// SessionMiddleware protege todas as rotas que exigem login.
func SessionMiddleware(baseRoute string, duration int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c, err := r.Cookie("session_token")
			if err != nil {
				http.Redirect(w, r, baseRoute+"/login", http.StatusSeeOther)
				return
			}
			claims := &Claims{}
			_, err = jwt.ParseWithClaims(c.Value, claims, func(t *jwt.Token) (interface{}, error) {
				return jwtKey, nil
			})
			if err != nil || claims.Scope != baseRoute || claims.ExpiresAt.Time.Before(time.Now()) {
				http.Redirect(w, r, baseRoute+"/login", http.StatusSeeOther)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// LoginHandler serve a página de login e gera o cookie JWT.
func LoginHandler(baseRoute string, duration int) http.Handler {
	// lê o template embarcado em internal/auth/templates/login.html
	tpl := template.Must(template.ParseFS(loginFS, "templates/login.html"))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			tpl.Execute(w, nil)
			return
		}
		user := r.FormValue("username")
		pass := r.FormValue("password")
		log.Printf("[LOG] login: user=%q", user)

		info, err := Authenticate(user, pass)
		if err != nil {
			log.Printf("LDAP auth failed for %q: %v", user, err)
			http.Error(w, "Invalid credentials", http.StatusUnauthorized)
			return
		}

		exp := time.Now().Add(time.Duration(duration) * time.Second)
		claims := &Claims{
			Username: info.Username,
			Scope:    baseRoute,
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(exp),
				Subject:   info.Username,
			},
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenStr, _ := token.SignedString(jwtKey)

		http.SetCookie(w, &http.Cookie{
			Name:     "session_token",
			Value:    tokenStr,
			Expires:  exp,
			HttpOnly: true,
			Path:     baseRoute,
		})
		http.Redirect(w, r, path.Clean(baseRoute)+"/", http.StatusSeeOther)
	})
}

// LogoutHandler limpa o cookie de sessão.
func LogoutHandler(baseRoute string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{
			Name:     "session_token",
			Value:    "",
			Path:     baseRoute,
			Expires:  time.Unix(0, 0),
			MaxAge:   -1,
			HttpOnly: true,
		})
		http.Redirect(w, r, baseRoute+"/login", http.StatusSeeOther)
	})
}
