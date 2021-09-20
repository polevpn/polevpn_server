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

	httpServer := NewHttpServer(upstream, downstream, requestHandler)
	httpServer.SetLoginCheckHandler(loginchecker)
	wg.Add(1)
	go httpServer.ListenTLS(wg,
		config.Get("endpoint.listen").AsStr(),
		config.Get("endpoint.cert_file").AsStr(),
		config.Get("endpoint.key_file").AsStr(),
		config.Get("endpoint.ws_path").AsStr(),
		config.Get("endpoint.h3_path").AsStr(),
	)
	elog.Infof("listen https %v,[websocket %v],[http3 %v]",
		config.Get("endpoint.listen").AsStr(),
		config.Get("endpoint.ws_path").AsStr(),
		config.Get("endpoint.h3_path").AsStr())

	wg.Wait()

	return nil
}
