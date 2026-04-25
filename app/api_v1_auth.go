package app

import (
	"crypto/rand"
	"path"
	"time"

	"github.com/UNHCSC/proxman/auth"
	"github.com/gofiber/fiber/v2"
)

var jwtSigningKey []byte = make([]byte, 64)

func init() {
	if _, err := rand.Read(jwtSigningKey); err != nil {
		appLog.Errorf("failed to generate JWT signing key: %v\n", err)
		panic(err)
	}
}

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
			})

			err = c.Redirect(redirect)
			return
		}
	}

	err = c.Render("login", fiber.Map{
		"Title":         "Login",
		"Description":   "Authenticate with your directory credentials to access Proxmox VE ACL.",
		"CanonicalPath": "/login",
		"BodyClass":     "login-page",
		"Redirect":      redirect,
		"LoginError":    err.Error(),
	}, "layout")
	return
}

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

func getStatus(c *fiber.Ctx) (err error) {
	var user *auth.AuthUser
	if user = auth.IsAuthenticated(c, jwtSigningKey); user != nil {
		err = c.SendStatus(fiber.StatusOK)
		return
	}

	err = c.SendStatus(fiber.StatusUnauthorized)
	return
}
