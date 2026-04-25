package auth

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

func resetAuthState() {
	usersLock.Lock()
	defer usersLock.Unlock()

	activeUsers = make(map[string]*AuthUser)
	userInjections = make(map[string]*UserInjection)
}

func TestAuthenticateWithUserInjection(t *testing.T) {
	resetAuthState()
	t.Cleanup(resetAuthState)

	AddUserInjection("alice", "secret", AuthPermsUser)

	user, err := Authenticate("alice", "secret")
	if err != nil {
		t.Fatalf("Authenticate returned error: %v", err)
	}
	if user.Username != "alice" {
		t.Fatalf("expected username alice, got %q", user.Username)
	}
	if user.Permissions() != AuthPermsUser {
		t.Fatalf("expected user permissions, got %s", user.Permissions())
	}
	if GetActiveUser("alice") == nil {
		t.Fatal("expected alice to be active")
	}
}

func TestIsAuthenticatedAcceptsBearerToken(t *testing.T) {
	resetAuthState()
	t.Cleanup(resetAuthState)

	secret := []byte("test-secret")
	AddUserInjection("alice", "secret", AuthPermsUser)

	user, err := Authenticate("alice", "secret")
	if err != nil {
		t.Fatalf("Authenticate returned error: %v", err)
	}

	token, err := user.Token.SignedString(secret)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	app := fiber.New()
	app.Get("/protected", func(c *fiber.Ctx) error {
		if IsAuthenticated(c, secret) == nil {
			return c.SendStatus(fiber.StatusUnauthorized)
		}
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test returned error: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestLogoutRemovesActiveUser(t *testing.T) {
	resetAuthState()
	t.Cleanup(resetAuthState)

	AddUserInjection("alice", "secret", AuthPermsUser)
	if _, err := Authenticate("alice", "secret"); err != nil {
		t.Fatalf("Authenticate returned error: %v", err)
	}

	Logout("alice")

	if GetActiveUser("alice") != nil {
		t.Fatal("expected alice to be logged out")
	}
}

func TestGetActiveUserExpiresSessions(t *testing.T) {
	resetAuthState()
	t.Cleanup(resetAuthState)

	activeUsers["alice"] = &AuthUser{
		Username: "alice",
		Expiry:   time.Now().Add(-time.Minute),
		perms:    AuthPermsUser,
	}

	if GetActiveUser("alice") != nil {
		t.Fatal("expected expired active user to be ignored")
	}
}
