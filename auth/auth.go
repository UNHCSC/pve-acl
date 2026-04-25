package auth

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/UNHCSC/organesson/config"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/z46-dev/golog"
)

var authLog *golog.Logger = golog.New().Prefix("[Please call auth.Init() with the main logger]", golog.BoldRed)

func Init(parentLog *golog.Logger) (err error) {
	authLog = parentLog.SpawnChild().Prefix("[AUTH]", golog.BoldYellow)
	return
}

type authPerms uint8

const (
	AuthPermsNone          authPerms = iota // No permissions, cannot log in
	AuthPermsUser                           // Can view but not edit
	AuthPermsAdministrator                  // Can do everything
)

const SessionDuration = 12 * time.Hour

func (p authPerms) String() string {
	switch p {
	case AuthPermsAdministrator:
		return "administrator"
	case AuthPermsUser:
		return "user"
	default:
		return "none"
	}
}

func authPermsFromString(value string) authPerms {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case AuthPermsAdministrator.String():
		return AuthPermsAdministrator
	case AuthPermsUser.String():
		return AuthPermsUser
	default:
		return AuthPermsNone
	}
}

type AuthUser struct {
	LDAPConn *LDAPConn
	Token    *jwt.Token
	Expiry   time.Time
	Username string
	perms    authPerms
}

func (user *AuthUser) Permissions() authPerms {
	if user.perms != AuthPermsNone {
		return user.perms
	}

	if user.LDAPConn == nil {
		return AuthPermsNone
	}

	if groups, err := user.LDAPConn.Groups(); err != nil || len(groups) == 0 {
		return AuthPermsNone
	} else {
		for _, gName := range config.Config.LDAP.AdminGroups {
			if slices.Contains(groups, gName) {
				user.perms = AuthPermsAdministrator
				return user.perms
			}
		}

		for _, gName := range config.Config.LDAP.UserGroups {
			if slices.Contains(groups, gName) {
				user.perms = AuthPermsUser
				return user.perms
			}
		}
	}

	user.perms = AuthPermsNone
	return user.perms
}

var activeUsers = make(map[string]*AuthUser)
var usersLock *sync.RWMutex = &sync.RWMutex{}

func GetActiveUser(username string) *AuthUser {
	usersLock.RLock()
	defer usersLock.RUnlock()

	if user, ok := activeUsers[username]; ok {
		if user.Expiry.After(time.Now()) {
			return user
		}
	}

	return nil
}

func RefreshToken(user *AuthUser) {
	user.Expiry = time.Now().Add(SessionDuration)
}

func WithAuth(w http.ResponseWriter, r *http.Request, jwtSecret []byte) bool {
	var authToken string

	if cookie, err := r.Cookie("Authorization"); err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return false
	} else {
		authToken = cookie.Value
	}

	if authToken == "" {
		w.WriteHeader(http.StatusUnauthorized)
		return false
	}

	parsedToken, err := jwt.Parse(authToken, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}

		return jwtSecret, nil
	})

	if err != nil || !parsedToken.Valid {
		w.WriteHeader(http.StatusUnauthorized)
		return false
	}

	if claims, ok := parsedToken.Claims.(jwt.MapClaims); ok {
		if username, ok := claims["username"].(string); ok {
			usersLock.Lock()
			defer usersLock.Unlock()

			user, ok := activeUsers[username]
			if ok && user.Expiry.After(time.Now()) {
				RefreshToken(user)
				return true
			}
		}
	}

	w.WriteHeader(http.StatusUnauthorized)
	return false
}

func IsAuthenticated(r *fiber.Ctx, jwtSecret []byte) *AuthUser {
	var authToken string = r.Cookies("Authorization")
	if authToken == "" {
		if header := r.Get("Authorization"); header != "" {
			authToken = strings.TrimSpace(strings.TrimPrefix(header, "Bearer"))
		}
		if authToken == "" {
			return nil
		}
	}

	parsedToken, err := jwt.Parse(authToken, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}

		return jwtSecret, nil
	})

	if err != nil || !parsedToken.Valid {
		return nil
	}

	if claims, ok := parsedToken.Claims.(jwt.MapClaims); ok {
		if username, ok := claims["username"].(string); ok {
			usersLock.RLock()
			user, ok := activeUsers[username]
			if ok && user.Expiry.After(time.Now()) {
				usersLock.RUnlock()
				return user
			}
			usersLock.RUnlock()

			expiry := time.Now().Add(SessionDuration)
			if expiresAt, err := claims.GetExpirationTime(); err == nil && expiresAt != nil {
				expiry = expiresAt.Time
			}
			if expiry.Before(time.Now()) {
				return nil
			}

			perms := AuthPermsNone
			if claimPerms, ok := claims["perms"].(string); ok {
				perms = authPermsFromString(claimPerms)
			}
			if perms == AuthPermsNone {
				return nil
			}

			user = &AuthUser{
				Token:    parsedToken,
				Expiry:   expiry,
				Username: username,
				perms:    perms,
			}

			usersLock.Lock()
			activeUsers[username] = user
			usersLock.Unlock()
			return user
		}
	}

	return nil
}

func Authenticate(username, password string) (*AuthUser, error) {
	if injection := GetUserInjection(username, password); injection != nil {
		expiry := time.Now().Add(SessionDuration)
		user := &AuthUser{
			LDAPConn: nil,
			Token: jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
				"username": username,
				"perms":    injection.Permissions.String(),
				"exp":      expiry.Unix(),
				"iat":      time.Now().Unix(),
			}),
			Expiry:   expiry,
			Username: username,
			perms:    injection.Permissions,
		}

		usersLock.Lock()
		defer usersLock.Unlock()
		activeUsers[username] = user
		return user, nil
	}

	ldapConn, err := NewLDAPConn(username, password)
	if err != nil {
		return nil, err
	}

	if !ldapConn.IsAuthenticated {
		ldapConn.Close()
		return nil, ErrUnauthorized
	}

	expiry := time.Now().Add(SessionDuration)
	user := &AuthUser{
		LDAPConn: ldapConn,
		Token:    jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"username": username, "exp": expiry.Unix(), "iat": time.Now().Unix()}),
		Expiry:   expiry,
		Username: username,
	}

	if user.Permissions() == AuthPermsNone {
		ldapConn.Close()
		return nil, fmt.Errorf("user is unauthorized to use this application")
	}
	user.Token = jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"username": username,
		"perms":    user.Permissions().String(),
		"exp":      expiry.Unix(),
		"iat":      time.Now().Unix(),
	})

	usersLock.Lock()
	defer usersLock.Unlock()

	activeUsers[username] = user
	authLog.Infof("User '%s' authenticated with permissions level %d.\n", username, user.Permissions())

	return user, nil
}

func Logout(username string) {
	usersLock.Lock()
	defer usersLock.Unlock()

	if user, ok := activeUsers[username]; ok {
		if user.LDAPConn != nil {
			user.LDAPConn.Close()
		}

		authLog.Infof("User '%s' logged out.\n", username)
		delete(activeUsers, username)
	}
}
