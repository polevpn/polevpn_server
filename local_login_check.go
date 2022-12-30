package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-ldap/ldap/v3"
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

func (llc *LocalLoginChecker) CheckLogin(user string, pwd string, remoteIp string, deviceType string, deviceId string) error {

	var err error

	if Config.Has("auth.file") {
		err = llc.checkFileLogin(user, pwd)
	}

	if err == nil {
		return nil
	}

	if Config.Has("auth.http") {
		err = llc.checkHttpLogin(user, pwd, deviceType, deviceId)
	}

	if err == nil {
		return nil
	}

	if Config.Has("auth.ldap") {
		err = llc.checkLDAPLogin(user, pwd)
	}

	return err

}

func (llc *LocalLoginChecker) checkFileLogin(user string, pwd string) error {

	filePath := Config.Get("auth.file.path").AsStr()

	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	br := bufio.NewReader(f)
	for {
		line, _, c := br.ReadLine()
		if c == io.EOF {
			break
		}
		userPassword := strings.Trim(string(line), "\n\r")
		userPasswordArr := strings.Split(userPassword, ",")
		if len(userPasswordArr) != 2 {
			continue
		}
		if userPasswordArr[0] == user && userPasswordArr[1] == pwd {
			return nil
		}
	}
	return errors.New("user or password incorrect")
}

func (llc *LocalLoginChecker) checkHttpLogin(user string, pwd string, deviceType string, deviceId string) error {

	req := anyvalue.New()

	req.Set("user", user)
	req.Set("pwd", pwd)
	req.Set("deviceType", deviceType)
	req.Set("deviceId", deviceId)

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

	l, err := ldap.DialURL(Config.Get("auth.ldap.host").AsStr())
	if err != nil {
		return err
	}
	defer l.Close()

	err = l.Bind(Config.Get("auth.ldap.admin_dn").AsStr(), Config.Get("auth.ldap.admin_pwd").AsStr())
	if err != nil {
		return err
	}

	searchRequest := ldap.NewSearchRequest(
		Config.Get("auth.ldap.user_dn").AsStr(),
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		fmt.Sprintf("(&(objectClass=organizationalPerson)(uid=%s))", user),
		[]string{"dn"},
		nil,
	)

	sr, err := l.Search(searchRequest)
	if err != nil {
		return err
	}

	if len(sr.Entries) != 1 {
		return errors.New("User does not exist")
	}

	userdn := sr.Entries[0].DN

	err = l.Bind(userdn, pwd)
	if err != nil {
		return err
	}

	return nil
}
