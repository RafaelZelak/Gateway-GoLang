package auth

import (
	"fmt"
	"strings"

	"github.com/go-ldap/ldap/v3"
)

// UserInfo contains username, displayName and groups from LDAP.
type UserInfo struct {
	Username    string
	DisplayName string
	Groups      []string
}

// Authenticate binds with user@domain, then searches displayName and memberOf.
func Authenticate(username, password string) (*UserInfo, error) {
	ldapHost := "webmin.digitalup.intranet"
	if ldapHost == "" {
		ldapHost = "digitalup.intranet"
	}
	ldapPort := "389"
	if ldapPort == "" {
		ldapPort = "389"
	}
	addr := fmt.Sprintf("%s:%s", ldapHost, ldapPort)

	// 2) Dial no endereço correto
	l, err := ldap.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("LDAP connection failed: %w", err)
	}
	defer l.Close()

	// 3) Bind com user@domain (o DOMAIN original ainda usado no userDN)
	domain := "digitalup.intranet"
	userDN := fmt.Sprintf("%s@%s", username, domain)
	if err := l.Bind(userDN, password); err != nil {
		return nil, fmt.Errorf("invalid credentials: %w", err)
	}

	// 4) Search etc… (permanece igual)
	baseDN := "DC=" + strings.ReplaceAll(domain, ".", ",DC=")
	filter := fmt.Sprintf("(sAMAccountName=%s)", ldap.EscapeFilter(username))
	req := ldap.NewSearchRequest(
		baseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		filter,
		[]string{"displayName", "memberOf"},
		nil,
	)

	sr, err := l.Search(req)
	if err != nil {
		return nil, fmt.Errorf("LDAP search failed: %w", err)
	}
	if len(sr.Entries) == 0 {
		return nil, fmt.Errorf("user not found")
	}

	entry := sr.Entries[0]
	display := entry.GetAttributeValue("displayName")
	if display == "" {
		display = username
	}

	var groups []string
	for _, dn := range entry.GetAttributeValues("memberOf") {
		parts := strings.SplitN(dn, ",", 2)
		cn := strings.TrimPrefix(parts[0], "CN=")
		groups = append(groups, cn)
	}

	return &UserInfo{
		Username:    username,
		DisplayName: display,
		Groups:      groups,
	}, nil
}
