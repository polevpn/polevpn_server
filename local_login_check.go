package main

import (
	"encoding/base64"
	"encoding/json"
)

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

	if len(pwd) > MAX_PASSWORD_LENGHT {
		return llc.checkLoginByToken(user, pwd)
	} else {
		return llc.checkLoginByPassword(user, pwd)
	}
}

func (llc *LocalLoginChecker) checkLoginByToken(user string, pwd string) bool {
	users := Config.Get("users").AsArray()

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

func (llc *LocalLoginChecker) checkLoginByPassword(user string, pwd string) bool {

	crypted, err := base64.StdEncoding.DecodeString(pwd)

	if err != nil {
		return false
	}

	origin, err := AesDecrypt(crypted, ServerAesKey)
	if err != nil {
		return false
	}

	userinfo := User{}
	json.Unmarshal(origin, &userinfo)

	if user != userinfo.Email {
		return false
	}

	return true
}
