package main

import (
	"sync"
	"time"

	"github.com/polevpn/elog"
)

const (
	CONNECTION_TIMEOUT    = 3
	CHECK_TIMEOUT_INTEVAL = 5
)

type WebSocketConnMgr struct {
	ip2conns    map[string]*WebSocketConn
	conn2ips    map[string]string
	ip2actives  map[string]time.Time
	ip2users    map[string]string
	conn2users  map[string]string
	mutex       *sync.Mutex
	addresspool *AddressPool
}

func NewWebSocketConnMgr() *WebSocketConnMgr {
	wscm := &WebSocketConnMgr{
		ip2conns:   make(map[string]*WebSocketConn),
		mutex:      &sync.Mutex{},
		conn2ips:   make(map[string]string),
		ip2actives: make(map[string]time.Time),
		ip2users:   make(map[string]string),
		conn2users: make(map[string]string),
	}
	go wscm.CheckTimeout()
	return wscm
}

func (wscm *WebSocketConnMgr) CheckTimeout() {
	for range time.NewTicker(time.Second * CHECK_TIMEOUT_INTEVAL).C {
		timeNow := time.Now()
		for ip, lastActive := range wscm.ip2actives {
			if timeNow.Sub(lastActive) > time.Minute*CONNECTION_TIMEOUT {
				wscm.RelelaseAddress(ip)
				conn, ok := wscm.ip2conns[ip]
				if ok {
					wscm.DetachIPAddress(conn)
					wscm.DetachUserFromConn(conn)
					conn.Close(false)
				}
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

	if wscm.addresspool == nil {
		elog.Error("address pool haven't set")
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

	if wscm.addresspool == nil {
		return
	}
	delete(wscm.ip2actives, ip)
	wscm.addresspool.Release(ip)
}

func (wscm *WebSocketConnMgr) IsAllocedAddress(ip string) bool {

	wscm.mutex.Lock()
	defer wscm.mutex.Unlock()

	if wscm.addresspool == nil {
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

func (wscm *WebSocketConnMgr) AttachUserToConn(user string, conn *WebSocketConn) {
	wscm.mutex.Lock()
	defer wscm.mutex.Unlock()
	wscm.conn2users[conn.String()] = user
}

func (wscm *WebSocketConnMgr) DetachUserFromConn(conn *WebSocketConn) {
	wscm.mutex.Lock()
	defer wscm.mutex.Unlock()
	delete(wscm.conn2users, conn.String())
}

func (wscm *WebSocketConnMgr) GetConnAttachUser(conn *WebSocketConn) string {
	wscm.mutex.Lock()
	defer wscm.mutex.Unlock()
	return wscm.conn2users[conn.String()]
}

func (wscm *WebSocketConnMgr) AttachUserToIP(user string, ip string) {
	wscm.mutex.Lock()
	defer wscm.mutex.Unlock()
	wscm.ip2users[ip] = user
}

func (wscm *WebSocketConnMgr) DetachUserFromIP(ip string) {
	wscm.mutex.Lock()
	defer wscm.mutex.Unlock()
	delete(wscm.ip2users, ip)
}

func (wscm *WebSocketConnMgr) GetIPAttachUser(ip string) string {
	wscm.mutex.Lock()
	defer wscm.mutex.Unlock()
	return wscm.ip2users[ip]
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
