package main

import "sync"

type WebSocketConnMgr struct {
	ip2conns map[string]*WebSocketConn
	mutex    *sync.Mutex
}

func NewWebSocketConnMgr() *WebSocketConnMgr {
	return &WebSocketConnMgr{ip2conns: make(map[string]*WebSocketConn), mutex: &sync.Mutex{}}
}

func (wscm *WebSocketConnMgr) AttachIPAddress(ip string, conn *WebSocketConn) {
	wscm.mutex.Lock()
	defer wscm.mutex.Unlock()
	wscm.ip2conns[ip] = conn
}

func (wscm *WebSocketConnMgr) DetachIPAddress(ip string, conn *WebSocketConn) {
	wscm.mutex.Lock()
	defer wscm.mutex.Unlock()
	delete(wscm.ip2conns, ip)
}

func (wscm *WebSocketConnMgr) GetWebSocketConn(ip string) *WebSocketConn {
	wscm.mutex.Lock()
	defer wscm.mutex.Unlock()
	return wscm.ip2conns[ip]
}
