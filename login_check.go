package main

type LoginChecker interface {
	CheckLogin(user string, pwd string) error
}
