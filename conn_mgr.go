package main

import (
	"sync"
	"time"
)

const (
	CONNECTION_TIMEOUT    = 3
	CHECK_TIMEOUT_INTEVAL = 5
)

type WebSocketConnMgr struct {
	ip2conns    map[string]*WebSocketConn
	conn2ips    map[string]string
	ip2actives  map[string]time.Time
	mutex       *sync.Mutex
	addresspool *AddressPool
}

func NewWebSocketConnMgr() *WebSocketConnMgr {
	wscm := &WebSocketConnMgr{
		ip2conns:   make(map[string]*WebSocketConn),
		mutex:      &sync.Mutex{},
		conn2ips:   make(map[string]string),
		ip2actives: make(map[string]time.Time),
	}
	go wscm.CheckTimeout()
	return wscm
}

func (wscm *WebSocketConnMgr) CheckTimeout() {
	for range time.NewTicker(time.Second * CHECK_TIMEOUT_INTEVAL).C {
		timeNow := time.Now()
		for ip, lastActive := range wscm.ip2actives {
			if timeNow.Sub(lastActive) > time.Minute*CONNECTION_TIMEOUT {
				wscm.mutex.Lock()
				defer wscm.mutex.Unlock()
				wscm.RelelaseAddress(ip)
				conn, ok := wscm.ip2conns[ip]
				if ok {
					wscm.DetachIPAddress(conn)
					conn.Close()
				}
				delete(wscm.ip2actives, ip)
			}
		}
	}
}

func (wscm *WebSocketConnMgr) SetAddressPool(addrespool *AddressPool) {
	wscm.addresspool = addrespool
}

func (wscm *WebSocketConnMgr) AllocAddress() string {

	wscm.mutex.Lock()
	defer wscm.mutex.Unlock()

	if wscm.addresspool != nil {
		return ""
	}
	ip := wscm.addresspool.Alloc()
	if ip != "" {
		wscm.ip2actives[ip] = time.Now()
	}
	return ip
}

func (wscm *WebSocketConnMgr) RelelaseAddress(ip string) {

	wscm.mutex.Lock()
	defer wscm.mutex.Unlock()

	if wscm.addresspool != nil {
		return
	}
	delete(wscm.ip2actives, ip)
	wscm.addresspool.Release(ip)
}

func (wscm *WebSocketConnMgr) IsAllocedAddress(ip string) bool {

	wscm.mutex.Lock()
	defer wscm.mutex.Unlock()

	if wscm.addresspool != nil {
		return false
	}
	return wscm.addresspool.IsAlloc(ip)
}

func (wscm *WebSocketConnMgr) UpdateConnActiveTime(conn *WebSocketConn) {

	wscm.mutex.Lock()
	defer wscm.mutex.Unlock()
	ip, ok := wscm.conn2ips[conn.String()]
	if ok {
		wscm.ip2actives[ip] = time.Now()
	}
	wscm.addresspool.Release(ip)
}

func (wscm *WebSocketConnMgr) AttachIPAddress(ip string, conn *WebSocketConn) {
	wscm.mutex.Lock()
	defer wscm.mutex.Unlock()
	wscm.ip2conns[ip] = conn
	wscm.conn2ips[conn.String()] = ip
}

func (wscm *WebSocketConnMgr) IsDetached(ip string) bool {
	wscm.mutex.Lock()
	defer wscm.mutex.Unlock()
	_, ok := wscm.ip2conns[ip]
	return ok
}

func (wscm *WebSocketConnMgr) DetachIPAddress(conn *WebSocketConn) {
	wscm.mutex.Lock()
	defer wscm.mutex.Unlock()
	ip, ok := wscm.conn2ips[conn.String()]
	if ok {
		delete(wscm.ip2conns, ip)
		delete(wscm.conn2ips, conn.String())
	}
}

func (wscm *WebSocketConnMgr) GetWebSocketConnByIP(ip string) *WebSocketConn {
	wscm.mutex.Lock()
	defer wscm.mutex.Unlock()
	return wscm.ip2conns[ip]
}

func (wscm *WebSocketConnMgr) GeIPByWebsocketConn(conn *WebSocketConn) string {
	wscm.mutex.Lock()
	defer wscm.mutex.Unlock()
	return wscm.conn2ips[conn.String()]
}
