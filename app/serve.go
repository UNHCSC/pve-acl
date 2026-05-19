package app

import (
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/UNHCSC/organesson/config"
	"github.com/gofiber/fiber/v2"
)

// runHttpRedirectServer starts a companion server that redirects plain HTTP to the configured app address.
func runHttpRedirectServer(address string, targetAddress string, useTLS bool) (err error) {
	var (
		redirectApp *fiber.App = fiber.New(fiber.Config{
			Network: "tcp",
		})
		targetPort string
	)

	if _, targetPort, err = net.SplitHostPort(targetAddress); err != nil {
		return
	}

	redirectApp.Use(func(c *fiber.Ctx) error {
		var targetScheme, host string = "http", net.JoinHostPort(trimIPv6HostBrackets(c.Hostname()), targetPort)

		if useTLS {
			targetScheme = "https"
		}

		if (useTLS && targetPort == "443") || (!useTLS && targetPort == "80") {
			host = c.Hostname()
		}

		return c.Redirect(targetScheme+"://"+host+c.OriginalURL(), fiber.StatusMovedPermanently)
	})

	go func() {
		var err error
		if err = redirectApp.Listen(address); err != nil {
			panic(err)
		}
	}()

	return
}

// trimIPv6HostBrackets removes square brackets from an IPv6 host string.
func trimIPv6HostBrackets(host string) (valueResult string) {
	return strings.TrimPrefix(strings.TrimSuffix(host, "]"), "[")
}

// StartApp starts the Fiber app using either discovered TLS credentials or plain HTTP.
func StartApp(app *fiber.App) (err error) {
	if len(config.Config.WebServer.RedirectServerAddresses) > 0 && len(config.Config.WebServer.RedirectServerAddresses[0]) > 0 {
		for _, redirectAddress := range config.Config.WebServer.RedirectServerAddresses {
			runHttpRedirectServer(redirectAddress, config.Config.WebServer.Address, config.Config.WebServer.TLSDir != "")
		}
	}

	if config.Config.WebServer.TLSDir != "" {
		var (
			certPath, keyPath string
			found             bool
		)

		if certPath, keyPath, found = discoverTLSKeys(config.Config.WebServer.TLSDir); !found {
			err = fiber.ErrInternalServerError
			return
		}

		err = app.ListenTLS(config.Config.WebServer.Address, certPath, keyPath)
		return
	}

	err = app.Listen(config.Config.WebServer.Address)
	return
}

// discoverTLSKeys finds a likely certificate and key pair in the configured TLS directory.
func discoverTLSKeys(dir string) (certPath, keyPath string, found bool) {
	type Candidate struct {
		cert string
		key  string
	}

	var (
		candidates []Candidate = []Candidate{
			{"fullchain.pem", "privkey.pem"},
			{"cert.pem", "key.pem"},
			{"tls.crt", "tls.key"},
			{"server.crt", "server.key"},
			{"webserver.crt", "webserver.key"},
		}
		crtFiles []string
		keyFiles []string
		entries  []os.DirEntry
		err      error
		name     string
	)

	for _, candidate := range candidates {
		certPath = filepath.Join(dir, candidate.cert)
		keyPath = filepath.Join(dir, candidate.key)

		if fileExists(certPath) && fileExists(keyPath) {
			return certPath, keyPath, true
		}
	}

	if entries, err = os.ReadDir(dir); err != nil {
		return "", "", false
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name = entry.Name()
		if strings.HasSuffix(name, ".crt") {
			crtFiles = append(crtFiles, filepath.Join(dir, name))
		}
		if strings.HasSuffix(name, ".key") {
			keyFiles = append(keyFiles, filepath.Join(dir, name))
		}
	}

	if len(crtFiles) > 0 && len(keyFiles) > 0 {
		return crtFiles[0], keyFiles[0], true
	}

	return "", "", false
}

// fileExists reports whether a path exists and is not a directory.
func fileExists(p string) (okResult bool) {
	var (
		info os.FileInfo
		err  error
	)

	info, err = os.Stat(p)
	if err != nil {
		return false
	}
	return !info.IsDir()
}
