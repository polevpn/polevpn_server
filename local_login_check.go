package main

import (
	"bytes"
	"errors"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/go-ldap/ldap"
	"github.com/polevpn/anyvalue"
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

func (llc *LocalLoginChecker) CheckLogin(user string, pwd string) error {

	var err error

	if Config.Has("auth.users") {
		err = llc.checkUserLogin(user, pwd)
	}

	if err == nil {
		return nil
	}

	if Config.Has("auth.http") {
		err = llc.checkHttpLogin(user, pwd)
	}

	if err == nil {
		return nil
	}

	if Config.Has("auth.ldap") {
		err = llc.checkLDAPLogin(user, pwd)
	}

	return err

}

func (llc *LocalLoginChecker) checkUserLogin(user string, pwd string) error {

	users := Config.Get("auth.users").AsArray()

	for _, u := range users {
		u, ok := u.(map[string]interface{})
		if ok {
			if u["user"].(string) == user && u["pwd"].(string) == pwd {
				return nil
			}
		}
	}
	return errors.New("user or password incorrect")
}

func (llc *LocalLoginChecker) checkHttpLogin(user string, pwd string) error {

	req := anyvalue.New()

	req.Set("user", user)
	req.Set("pwd", pwd)
	data, _ := req.EncodeJson()

	client := http.Client{Timeout: time.Duration(Config.Get("auth.http.timeout").AsInt()) * time.Second}
	request, err := http.NewRequest(http.MethodPost, Config.Get("auth.http.url").AsStr(), bytes.NewReader(data))

	if err != nil {
		return err
	}

	resp, err := client.Do(request)

	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil
		}
		return errors.New(string(data))
	}
	return nil
}

func (llc *LocalLoginChecker) checkLDAPLogin(user string, pwd string) error {

	client, err := ldap.DialURL(Config.Get("auth.ldap.host").AsStr())
	if err != nil {
		return err
	}
	defer client.Close()

	_, err = client.SimpleBind(&ldap.SimpleBindRequest{
		Username: "CN=" + user + "," + Config.Get("auth.ldap.dn").AsStr(),
		Password: pwd,
	})
	if err != nil {
		return err
	}

	return nil
}
