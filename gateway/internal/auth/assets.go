// internal/auth/assets.go

package auth

import "embed"

// loginFS embute o template de login universal.
//go:embed templates/login.html
var loginFS embed.FS // :contentReference[oaicite:2]{index=2}
