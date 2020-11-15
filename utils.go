package main

import (
	"os"
	"runtime/debug"

	"github.com/polevpn/anyvalue"
	"github.com/polevpn/elog"
)

func GetConfig(configfile string) (*anyvalue.AnyValue, error) {

	f, err := os.Open(configfile)
	if err != nil {
		return nil, err
	}
	return anyvalue.NewFromJsonReader(f)
}

func PanicHandler() {
	if err := recover(); err != nil {
		elog.Error("Panic Exception:", err)
		elog.Error(string(debug.Stack()))
	}
}

func PanicHandlerExit() {
	if err := recover(); err != nil {
		elog.Error("Panic Exception:", err)
		elog.Error(string(debug.Stack()))
		elog.Error("************Program Exit************")
		elog.Flush()
		os.Exit(0)
	}
}
