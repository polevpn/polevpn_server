package main

type LoginChecker interface {
	CheckLogin(user string, pwd string, remoteIp string, deviceType string, deviceId string) error
}
