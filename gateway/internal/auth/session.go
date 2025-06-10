package auth

import (
	"html/template"
	"log"
	"net/http"
	"path"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims holds username and scope.
type Claims struct {
	Username string `json:"username"`
	Scope    string `json:"scope"`
	jwt.RegisteredClaims
}

// SessionMiddleware protects the service routes.
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

func LoginHandler(baseRoute string, duration int) http.Handler {
	// this reads embedded internal/auth/templates/login.html
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

// LogoutHandler clears the session and redirects to login.
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
