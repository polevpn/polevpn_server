package main

const (
	MAX_PASSWORD_LENGHT = 16
)

type User struct {
	Uid           uint64
	Email         string
	Vip           int
	VipExpireTime uint64
	LastLoginTime uint64
}

type LocalLoginChecker struct {
}

func NewLocalLoginChecker() *LocalLoginChecker {
	return &LocalLoginChecker{}
}

func (llc *LocalLoginChecker) CheckLogin(user string, pwd string) bool {

	return llc.checkLoginByPassword(user, pwd)
}

func (llc *LocalLoginChecker) checkLoginByPassword(user string, pwd string) bool {
	users := Config.Get("auth.users").AsArray()

	for _, u := range users {
		u, ok := u.(map[string]interface{})
		if ok {
			if u["user"].(string) == user && u["pwd"].(string) == pwd {
				return true
			}
		}
	}
	return false
}
