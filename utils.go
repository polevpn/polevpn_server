package main

import (
	"os"
	"runtime/debug"

	"github.com/polevpn/elog"
)

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
