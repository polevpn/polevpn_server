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

	connmgr := NewConnMgr()

	connmgr.SetAddressPool(addresspool)

	packetHandler := NewPacketDispatcher()

	packetHandler.SetConnMgr(connmgr)

	tunio, err := NewTunIO(CH_TUNIO_WRITE_SIZE, packetHandler)

	if err != nil {
		elog.Error("create tun fail", err)
		return err
	}

	gwip := addresspool.GatewayIP()

	elog.Infof("set tun device ip %v", gwip)
	err = tunio.SetIPAddress(gwip)
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
	elog.Infof("add route %v to %v", addresspool.GetNetwork(), gwip)
	err = tunio.AddRoute(addresspool.GetNetwork(), gwip)
	if err != nil {
		elog.Error("set tun route fail", err)
		return err
	}

	tunio.StartProcess()

	loginchecker := NewLocalLoginChecker()
	wsRequestHandler := NewWSRequestHandler()
	wsRequestHandler.SetTunIO(tunio)
	wsRequestHandler.SetConnMgr(connmgr)

	h2RequestHandler := NewH2RequestHandler()
	h2RequestHandler.SetTunIO(tunio)
	h2RequestHandler.SetConnMgr(connmgr)

	upstream := config.Get("upstream_traffic_limit").AsUint64()
	downstream := config.Get("downstream_traffic_limit").AsUint64()

	httpServer := NewHttpServer(upstream, downstream, wsRequestHandler, h2RequestHandler)
	httpServer.SetLoginCheckHandler(loginchecker)

	elog.Infof("listen %v,ws %v,h2 %v", config.Get("http.listen").AsStr(), config.Get("http.ws_path").AsStr(), config.Get("http.h2_path").AsStr())

	if config.Get("http.tls_mode").AsBool() == true {
		err = httpServer.ListenTLS(
			config.Get("http.listen").AsStr(),
			config.Get("http.cert_file").AsStr(),
			config.Get("http.key_file").AsStr(),
			config.Get("http.ws_path").AsStr(),
			config.Get("http.h2_path").AsStr(),
		)

	} else {
		err = httpServer.Listen(config.Get("http.listen").AsStr(), config.Get("http.ws_path").AsStr(), config.Get("http.h2_path").AsStr())
	}

	if err != nil {
		elog.Error("http listen error", err)
		return err
	}
	return nil
}
