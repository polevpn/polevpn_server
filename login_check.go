package main

type LoginChecker interface {
	CheckLogin(user string, pwd string, deviceType string, deviceId string) error
}
