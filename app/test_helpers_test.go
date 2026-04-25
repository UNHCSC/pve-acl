package app

import (
	"testing"

	"github.com/UNHCSC/organesson/auth"
)

func authenticateTestUser(t *testing.T, username string, admin bool) string {
	t.Helper()

	perms := auth.AuthPermsUser
	if admin {
		perms = auth.AuthPermsAdministrator
	}

	auth.AddUserInjection(username, "secret", perms)
	t.Cleanup(func() {
		auth.DeleteUserInjection(username)
		auth.Logout(username)
	})

	user, err := auth.Authenticate(username, "secret")
	if err != nil {
		t.Fatalf("Authenticate returned error: %v", err)
	}

	token, err := user.Token.SignedString(jwtSigningKey)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	return token
}
