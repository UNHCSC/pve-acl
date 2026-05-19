package app

import (
	"time"

	"github.com/gofiber/fiber/v2"
)

// getHome renders the public home page.
func getHome(c *fiber.Ctx) (err error) {
	err = c.Render("home", fiber.Map{
		"Title":         "Organesson Cloud",
		"Description":   "A private cloud platform for projects, quotas, workloads, and delegated access control.",
		"CanonicalPath": "/",
		"BodyClass":     "home-page",
		"CurrentYear":   time.Now().Year(),
	}, "layout")
	return
}

// getDashboard renders the authenticated dashboard shell.
func getDashboard(c *fiber.Ctx) (err error) {
	err = c.Render("dashboard", fiber.Map{
		"Title":         "Dashboard",
		"Description":   "Manage projects, quotas, workloads, and access in Organesson Cloud.",
		"CanonicalPath": "/dashboard",
		"BodyClass":     "dashboard-page",
	}, "layout")
	return
}

// getLogin renders the login page with an optional redirect target.
func getLogin(c *fiber.Ctx) (err error) {
	err = c.Render("login", fiber.Map{
		"Title":         "Login",
		"Description":   "Authenticate with your directory credentials to access Organesson Cloud.",
		"CanonicalPath": "/login",
		"BodyClass":     "login-page",
		"Redirect":      c.Query("redirect", "/dashboard"),
	}, "layout")
	return
}
