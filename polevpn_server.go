package main

import (
	"sync"

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
	requestHandler := NewRequestHandler()
	requestHandler.SetTunIO(tunio)
	requestHandler.SetConnMgr(connmgr)

	upstream := config.Get("upstream_traffic_limit").AsUint64()
	downstream := config.Get("downstream_traffic_limit").AsUint64()

	wg := &sync.WaitGroup{}

	if config.Get("kcp.enable").AsBool() {
		kcpServer := NewKCPServer(upstream, downstream, requestHandler)
		kcpServer.SetLoginCheckHandler(loginchecker)
		wg.Add(1)
		go kcpServer.Listen(wg, config.Get("kcp.listen").AsStr())
		elog.Infof("listen kcp %v", config.Get("kcp.listen").AsStr())
	}

	if config.Get("http.enable").AsBool() {
		httpServer := NewHttpServer(upstream, downstream, requestHandler)
		httpServer.SetLoginCheckHandler(loginchecker)
		wg.Add(1)
		go httpServer.Listen(wg,
			config.Get("http.listen").AsStr(),
			config.Get("http.ws_path").AsStr(),
			config.Get("http.h2_path").AsStr(),
			config.Get("http.hc_path").AsStr(),
		)
		elog.Infof("listen http %v,[websocket %v],[http2 %v],[http1 %v]",
			config.Get("http.listen").AsStr(),
			config.Get("http.ws_path").AsStr(),
			config.Get("http.h2_path").AsStr(),
			config.Get("http.hc_path").AsStr())
	}

	if config.Get("http.tls_enable").AsBool() {
		httpServer := NewHttpServer(upstream, downstream, requestHandler)
		httpServer.SetLoginCheckHandler(loginchecker)
		wg.Add(1)
		go httpServer.ListenTLS(wg,
			config.Get("http.tls_listen").AsStr(),
			config.Get("http.cert_file").AsStr(),
			config.Get("http.key_file").AsStr(),
			config.Get("http.ws_path").AsStr(),
			config.Get("http.h2_path").AsStr(),
			config.Get("http.hc_path").AsStr(),
		)
		elog.Infof("listen https %v,[websocket %v],[http2 %v],[http1 %v]",
			config.Get("http.tls_listen").AsStr(),
			config.Get("http.ws_path").AsStr(),
			config.Get("http.h2_path").AsStr(),
			config.Get("http.hc_path").AsStr())
	}

	wg.Wait()

	return nil
}
