//go:build linux || darwin

package hostfacts

import (
	"os"
	"os/user"
	"strconv"
)

func observeEffectiveUser() (string, error) {
	// Current caches the real user. LookupId instead interrogates the account
	// database for the effective uid observed on every call, as whoami does.
	account, err := user.LookupId(strconv.Itoa(os.Geteuid()))
	if err != nil {
		return "", err
	}
	return account.Username, nil
}
