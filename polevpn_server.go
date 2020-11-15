package main

import (
	"github.com/polevpn/anyvalue"
	"github.com/polevpn/elog"
)

type PoleVPNServer struct {
}

func NewPoleVPNServer() *PoleVPNServer {
	return &PoleVPNServer{}
}

func (ps *PoleVPNServer) Start(config *anyvalue.AnyValue) error {
	var err error

	addresspool, err := NewAddressPool(config.Get("network_cidr").AsStr())

	if err != nil {
		elog.Error("new address pool", err)
		return err
	}

	connmgr := NewWebSocketConnMgr()

	connmgr.SetAddressPool(addresspool)

	packetHandler := NewPacketDispatcher()

	packetHandler.SetWebSocketConnMgr(connmgr)

	tunio, err := NewTunIO(CH_TUNIO_WRITE_SIZE, packetHandler)

	if err != nil {
		elog.Error("create tun fail", err)
		return err
	}

	gwip1 := addresspool.GatewayIP1()
	gwip2 := addresspool.GatewayIP2()

	elog.Infof("set tun device ip src:%v,ip dst: %v", gwip1, gwip2)
	err = tunio.SetIPAddress(gwip1, gwip2)
	if err != nil {
		elog.Error("set tun ip address fail", err)
		return err
	}

	elog.Info("enable tun device")
	err = tunio.Enanble()
	if err != nil {
		elog.Error("enable tun fail", err)
		return err
	}
	elog.Infof("add route %v to %v", addresspool.GetNetwork(), gwip2)
	err = tunio.AddRoute(addresspool.GetNetwork(), gwip2)
	if err != nil {
		elog.Error("set tun route fail", err)
		return err
	}

	tunio.StartProcess()

	loginchecker := NewLocalLoginChecker()
	requestHandler := NewRequestDispatcher()
	requestHandler.SetTunIO(tunio)
	requestHandler.SetWebSocketConnMgr(connmgr)

	upstream := config.Get("upstream_traffic_limit").AsUint64()
	downstream := config.Get("downstream_traffic_limit").AsUint64()

	wserver := NewWebSocketServer(upstream, downstream, requestHandler)
	wserver.SetLoginCheckHandler(loginchecker)

	elog.Infof("listen %v,endpoint %v", config.Get("listen").AsStr(), config.Get("endpoint").AsStr())

	if config.Get("tls_mode").AsBool() == true {
		err = wserver.ListenTLS(
			config.Get("listen").AsStr(),
			config.Get("cert_file").AsStr(),
			config.Get("key_file").AsStr(),
			config.Get("endpoint").AsStr(),
		)

	} else {
		err = wserver.Listen(config.Get("listen").AsStr(), config.Get("endpoint").AsStr())
	}

	if err != nil {
		elog.Error("http listen error", err)
		return err
	}
	return nil
}
