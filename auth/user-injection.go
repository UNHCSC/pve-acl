package auth

// WARNING: User injection should never be done in prod. This lets you inject credentials of "username:password:permission level" into the system for testing ONLY.
type UserInjection struct {
	Username    string
	Password    string
	Permissions authPerms
}

var userInjections map[string]*UserInjection = make(map[string]*UserInjection)

// WARNING: User injection should never be done in prod. This lets you inject credentials of "username:password:permission level" into the system for testing ONLY.
func AddUserInjection(username, password string, perms authPerms) {
	userInjections[username] = &UserInjection{
		Username:    username,
		Password:    password,
		Permissions: perms,
	}

	authLog.Warningf("User injection added for username '%s' with permissions level %d. DO NOT USE THIS IN PRODUCTION!\n", username, perms)
}

// WARNING: User injection should never be done in prod. This lets you inject credentials of "username:password:permission level" into the system for testing ONLY.
func DeleteUserInjection(username string) {
	delete(userInjections, username)
	authLog.Warningf("User injection deleted for username '%s'.\n", username)
}

// WARNING: User injection should never be done in prod. This lets you inject credentials of "username:password:permission level" into the system for testing ONLY.
func GetUserInjection(username, password string) *UserInjection {
	if injection, ok := userInjections[username]; ok {
		if injection.Password == password {
			authLog.Warningf("User injection used for username '%s' with permissions level %d. DO NOT USE THIS IN PRODUCTION!\n", username, injection.Permissions)
			return injection
		}
	}

	return nil
}
