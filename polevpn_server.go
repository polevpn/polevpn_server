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
	bindips := make(map[string]string)
	bindiparr := config.Get("bind_ips").AsArray()
	for _, bindip := range bindiparr {
		bindip, ok := bindip.(map[string]interface{})
		if ok {
			bindips[bindip["user"].(string)] = bindip["ip"].(string)
		}
	}

	addresspool, err := NewAddressPool(config.Get("network_cidr").AsStr(), bindips)

	if err != nil {
		elog.Error("new address pool,", err)
		return err
	}

	routermgr := NewRouterMgr()
	routes := config.Get("server_routes").AsArray()
	for _, route := range routes {
		route, ok := route.(map[string]interface{})
		if ok {
			routermgr.AddRoute(route["cidr"].(string), route["gw"].(string))
		}
	}

	connmgr := NewConnMgr()

	connmgr.SetAddressPool(addresspool)

	packetHandler := NewPacketDispatcher()

	packetHandler.SetConnMgr(connmgr)

	tunio, err := NewTunIO(CH_TUNIO_WRITE_SIZE, packetHandler)

	if err != nil {
		elog.Error("create tun fail,", err)
		return err
	}

	gwip := addresspool.GatewayIP()

	elog.Infof("set tun device ip %v", gwip)
	err = tunio.SetIPAddress(gwip)
	if err != nil {
		elog.Error("set tun ip address fail,", err)
		return err
	}

	elog.Info("enable tun device")
	err = tunio.Enanble()
	if err != nil {
		elog.Error("enable tun fail,", err)
		return err
	}
	elog.Infof("add route %v to %v", addresspool.GetNetwork(), gwip)
	err = tunio.AddRoute(addresspool.GetNetwork(), gwip)
	if err != nil {
		elog.Error("set tun route fail,", err)
		return err
	}

	tunio.StartProcess()

	loginchecker := NewLocalLoginChecker()
	requestHandler := NewRequestHandler()
	requestHandler.SetTunIO(tunio)
	requestHandler.SetConnMgr(connmgr)
	requestHandler.SetRouterMgr(routermgr)

	upstream := config.Get("up_traffic_limit").AsUint64()
	downstream := config.Get("down_traffic_limit").AsUint64()

	wg := &sync.WaitGroup{}

	httpServer := NewHttpServer(upstream, downstream, requestHandler)
	httpServer.SetLoginCheckHandler(loginchecker)
	wg.Add(1)
	go httpServer.ListenTLS(wg,
		config.Get("endpoint.listen").AsStr(),
		config.Get("endpoint.cert_file").AsStr(),
		config.Get("endpoint.key_file").AsStr(),
	)
	elog.Infof("listen https at %v", config.Get("endpoint.listen").AsStr())

	wg.Wait()

	return nil
}
