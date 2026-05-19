package app

import (
	"testing"

	"github.com/UNHCSC/organesson/auth"
	"github.com/UNHCSC/organesson/db"
	"github.com/gofiber/fiber/v2"
)

func authenticateTestUser(t *testing.T, username string, admin bool) (valueResult string) {
	t.Helper()

	var (
		perms = auth.AuthPermsUser
		user  *auth.AuthUser
		token string
		err   error
	)

	if admin {
		perms = auth.AuthPermsAdministrator
	}

	auth.AddUserInjection(username, "secret", perms)
	t.Cleanup(func() {
		auth.DeleteUserInjection(username)
		auth.Logout(username)
	})

	if user, err = auth.Authenticate(username, "secret"); err != nil {
		t.Fatalf("Authenticate returned error: %v", err)
	}

	if token, err = user.Token.SignedString(jwtSigningKey); err != nil {
		t.Fatalf("sign token: %v", err)
	}

	return token
}

func ensureInitialSetupForTest(t *testing.T) {
	t.Helper()

	var err error
	if err = db.EnsureInitialSetup(); err != nil {
		t.Fatalf("EnsureInitialSetup returned error: %v", err)
	}
}

func newAuthenticatedFiberApp() (fiberApp *fiber.App) {
	fiberApp = fiber.New()
	fiberApp.Use(requireAPIAuth)
	return
}
