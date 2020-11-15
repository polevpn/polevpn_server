package main

import "github.com/polevpn/elog"

type LocalLoginChecker struct {
}

func NewLocalLoginChecker() *LocalLoginChecker {
	return &LocalLoginChecker{}
}

func (llc *LocalLoginChecker) CheckLogin(user string, pwd string) bool {
	users := Config.Get("users").AsArray()

	for _, u := range users {
		u, ok := u.(map[string]interface{})
		if ok {
			if u["user"].(string) == user && u["pwd"].(string) == pwd {
				return true
			}
		} else {
			elog.Println("xxxxxxx")
		}
	}
	return false
}
