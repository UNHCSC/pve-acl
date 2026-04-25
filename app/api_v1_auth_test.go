package app

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/UNHCSC/proxman/auth"
	"github.com/gofiber/fiber/v2"
)

func TestPostLoginSetsSiteWideCookie(t *testing.T) {
	auth.AddUserInjection("cookie-user", "secret", auth.AuthPermsUser)
	t.Cleanup(func() {
		auth.DeleteUserInjection("cookie-user")
		auth.Logout("cookie-user")
	})

	fiberApp := fiber.New()
	fiberApp.Post("/api/v1/auth/login", postLogin)

	form := url.Values{}
	form.Set("username", "cookie-user")
	form.Set("password", "secret")
	form.Set("redirect", "/dashboard")

	req := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := fiberApp.Test(req)
	if err != nil {
		t.Fatalf("login route returned error: %v", err)
	}
	if resp.StatusCode != fiber.StatusFound {
		t.Fatalf("expected 302, got %d", resp.StatusCode)
	}

	var authCookie string
	for _, cookie := range resp.Header.Values("Set-Cookie") {
		if strings.HasPrefix(cookie, "Authorization=") {
			authCookie = cookie
			break
		}
	}
	if authCookie == "" {
		t.Fatal("expected Authorization cookie")
	}
	if !strings.Contains(strings.ToLower(authCookie), "path=/") {
		t.Fatalf("expected site-wide cookie path, got %q", authCookie)
	}
	if !strings.Contains(authCookie, "HttpOnly") {
		t.Fatalf("expected HttpOnly cookie, got %q", authCookie)
	}
}
