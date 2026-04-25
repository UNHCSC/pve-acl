package app

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/UNHCSC/proxman/config"
	"github.com/gofiber/fiber/v2"
	"github.com/z46-dev/golog"
)

func TestInitAndListenProtectsAPIExceptAuthRoutes(t *testing.T) {
	config.Config = config.Configuration{}

	fiberApp, err := InitAndListen(golog.New())
	if err != nil {
		t.Fatalf("InitAndListen returned error: %v", err)
	}

	authResp, err := fiberApp.Test(httptest.NewRequest("GET", "/api/v1/auth/status", nil))
	if err != nil {
		t.Fatalf("auth status route returned error: %v", err)
	}
	if authResp.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("expected public auth status route to return auth result 401, got %d", authResp.StatusCode)
	}

	enumResp, err := fiberApp.Test(httptest.NewRequest("GET", "/api/v1/enums/asset-types", nil))
	if err != nil {
		t.Fatalf("enum route returned error: %v", err)
	}
	if enumResp.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("expected protected enum route to require auth with 401, got %d", enumResp.StatusCode)
	}
}

func TestInitAndListenSetsSecurityHeaders(t *testing.T) {
	config.Config = config.Configuration{}

	fiberApp, err := InitAndListen(golog.New())
	if err != nil {
		t.Fatalf("InitAndListen returned error: %v", err)
	}

	resp, err := fiberApp.Test(httptest.NewRequest("GET", "/dashboard", nil))
	if err != nil {
		t.Fatalf("dashboard route returned error: %v", err)
	}

	csp := resp.Header.Get("Content-Security-Policy")
	if csp == "" {
		t.Fatal("expected Content-Security-Policy header")
	}
	if !strings.Contains(csp, "script-src 'self'") {
		t.Fatalf("expected script-src self policy, got %q", csp)
	}
}

func TestPageTemplatesRenderReactRoots(t *testing.T) {
	config.Config = config.Configuration{}

	fiberApp, err := InitAndListen(golog.New())
	if err != nil {
		t.Fatalf("InitAndListen returned error: %v", err)
	}

	tests := map[string]string{
		"/":          `id="home-root"`,
		"/login":     `id="login-root"`,
		"/dashboard": `id="dashboard-root"`,
	}
	for path, marker := range tests {
		resp, err := fiberApp.Test(httptest.NewRequest("GET", path, nil))
		if err != nil {
			t.Fatalf("%s route returned error: %v", path, err)
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("read %s response: %v", path, err)
		}
		if !strings.Contains(string(body), marker) {
			t.Fatalf("expected %s response to contain %q", path, marker)
		}
	}
}
