package app

import (
	"crypto/rand"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/UNHCSC/organesson/auth"
	"github.com/UNHCSC/organesson/config"
	"github.com/gofiber/fiber/v2"
)

var jwtSigningKey []byte = make([]byte, 64)

func init() {
	var err error

	if _, err = rand.Read(jwtSigningKey); err != nil {
		appLog.Errorf("failed to generate JWT signing key: %v\n", err)
		panic(err)
	}
}

// initPersistentJWTSigningKey loads or creates the JWT signing key beside the configured database.
func initPersistentJWTSigningKey() (err error) {
	var dbPath string = config.Config.Database.File
	if dbPath == "" {
		return
	}

	var keyDir string = filepath.Dir(dbPath)
	if keyDir == "" || keyDir == "." {
		keyDir = "."
	}

	var keyPath string = filepath.Join(keyDir, ".organesson-session-key")

	var key []byte
	if key, err = os.ReadFile(keyPath); err == nil && len(key) >= 32 {
		jwtSigningKey = key
		return
	}

	key = make([]byte, 64)
	if _, err = rand.Read(key); err != nil {
		return fmt.Errorf("generate session key: %w", err)
	}

	if err = os.WriteFile(keyPath, key, 0600); err != nil {
		return fmt.Errorf("write session key: %w", err)
	}

	jwtSigningKey = key
	return
}

// postLogin authenticates a user and stores the session token in a site-wide cookie.
func postLogin(c *fiber.Ctx) (err error) {
	var (
		username, password, redirect string = c.FormValue("username"), c.FormValue("password"), c.FormValue("redirect")
		user                         *auth.AuthUser
		token                        string
	)

	if redirect == "" {
		redirect = "/"
	}

	redirect = path.Clean("/" + redirect)

	if user, err = auth.Authenticate(username, password); err == nil {
		if token, err = user.Token.SignedString(jwtSigningKey); err == nil {
			c.Cookie(&fiber.Cookie{
				Name:     "Authorization",
				Value:    token,
				Path:     "/",
				HTTPOnly: true,
				SameSite: "Lax",
				Expires:  user.Expiry,
				MaxAge:   int(time.Until(user.Expiry).Seconds()),
			})

			err = c.Redirect(redirect)
			return
		}
	}

	err = c.Render("login", fiber.Map{
		"Title":         "Login",
		"Description":   "Authenticate with your directory credentials to access Organesson Cloud.",
		"CanonicalPath": "/login",
		"BodyClass":     "login-page",
		"Redirect":      redirect,
		"LoginError":    err.Error(),
	}, "layout")
	return
}

// postLogout clears the current session cookie and logs out the authenticated user.
func postLogout(c *fiber.Ctx) (err error) {
	var user *auth.AuthUser
	if user = auth.IsAuthenticated(c, jwtSigningKey); user != nil {
		auth.Logout(user.Username)
	}

	// Must replace cookie as some browsers require a valid replacement before deletion
	c.Cookie(&fiber.Cookie{
		Name:    "Authorization",
		Value:   "",
		Path:    "/",
		Expires: time.Now().Add(-time.Hour),
	})

	return
}

// getStatus reports whether the current request has a valid authenticated session.
func getStatus(c *fiber.Ctx) (err error) {
	var user *auth.AuthUser
	if user = auth.IsAuthenticated(c, jwtSigningKey); user != nil {
		err = c.SendStatus(fiber.StatusOK)
		return
	}

	err = c.SendStatus(fiber.StatusUnauthorized)
	return
}
