package app

import (
	"net/http/httptest"
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
