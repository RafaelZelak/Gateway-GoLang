// gateway/internal/jobs/dns.go
package jobs

import (
	"os"
	"strings"
)

// EnsureResolvConf appends a public DNS nameserver (8.8.8.8) em /etc/resolv.conf
// se ainda não estiver presente, para que processos filhos consigam resolver hosts externos.
func EnsureResolvConf() {
	const fallback = "nameserver 8.8.8.8"
	data, err := os.ReadFile("/etc/resolv.conf")
	if err != nil {
		return
	}
	if !strings.Contains(string(data), fallback) {
		f, err := os.OpenFile("/etc/resolv.conf", os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return
		}
		defer f.Close()
		f.WriteString("\n" + fallback + "\n")
	}
}
