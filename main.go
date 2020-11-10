package main

import (
	"flag"

	"github.com/polevpn/elog"
)

func main() {

	flag.Parse()
	defer elog.Flush()

	var err error

	connmgr := NewWebSocketConnMgr()
	packetHandler := NewPacketDispatcher(100, connmgr)

	tunio, err := NewTunIO(1024, packetHandler)

	if err != nil {
		elog.Error("create tun fail", err)
		return
	}
	err = tunio.SetIPAddress("10.8.0.1", "10.8.0.2")
	if err != nil {
		elog.Error("set tun ip address fail", err)
		return
	}

	err = tunio.Enanble()
	if err != nil {
		elog.Error("enable tun fail", err)
		return
	}

	err = tunio.AddRoute("10.8.0.0/16", "10.8.0.2")
	if err != nil {
		elog.Error("set tun route fail", err)
		return
	}

	wserver := NewWebSocketServer(NewRequestDispatcher(tunio))

	err = wserver.Listen("0.0.0.0:8080", "/ws")

	if err != nil {
		elog.Error("http listen error", err)
	}
}
