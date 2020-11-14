package main

import (
	"flag"

	"github.com/polevpn/elog"
)

const (
	CH_TUNIO_WRITE_SIZE = 4096
)

func main() {

	flag.Parse()
	defer elog.Flush()

	var err error

	network := "10.8.0.0/16"

	addresspool, err := NewAddressPool(network)

	if err != nil {
		elog.Error("new address pool", err)
		return
	}

	connmgr := NewWebSocketConnMgr()

	packetHandler := NewPacketDispatcher()

	packetHandler.SetWebSocketConnMgr(connmgr)

	tunio, err := NewTunIO(CH_TUNIO_WRITE_SIZE, packetHandler)

	if err != nil {
		elog.Error("create tun fail", err)
		return
	}

	gwip1 := addresspool.GatewayIP1()
	gwip2 := addresspool.GatewayIP2()

	elog.Infof("set tun ip src:%v,ip dst: %v", gwip1, gwip2)
	err = tunio.SetIPAddress(gwip1, gwip2)
	if err != nil {
		elog.Error("set tun ip address fail", err)
		return
	}

	elog.Info("enable tun device")
	err = tunio.Enanble()
	if err != nil {
		elog.Error("enable tun fail", err)
		return
	}
	elog.Infof("add route %v to %v", addresspool.GetNetwork(), gwip2)
	err = tunio.AddRoute(addresspool.GetNetwork(), gwip2)
	if err != nil {
		elog.Error("set tun route fail", err)
		return
	}

	go tunio.Read()
	go tunio.Write()

	requestHandler := NewRequestDispatcher()
	requestHandler.SetTunIO(tunio)
	requestHandler.SetWebSocketConnMgr(connmgr)

	wserver := NewWebSocketServer(requestHandler)

	elog.Infof("listen to %v", "0.0.0.0:8080")
	err = wserver.Listen("0.0.0.0:8080", "/ws")

	if err != nil {
		elog.Error("http listen error", err)
	}
}
