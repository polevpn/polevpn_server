package main

import (
	"sync"
	"time"

	"github.com/polevpn/elog"
)

const (
	CONNECTION_TIMEOUT    = 1
	CHECK_TIMEOUT_INTEVAL = 5
)

type ConnMgr struct {
	ip2conns    map[string]Conn
	conns       map[string]Conn
	conn2ips    map[string]string
	ip2actives  map[string]time.Time
	ip2users    map[string]string
	conn2users  map[string]string
	mutex       *sync.Mutex
	addresspool *AddressPool
}

func NewConnMgr() *ConnMgr {
	cm := &ConnMgr{
		ip2conns:   make(map[string]Conn),
		conns:      make(map[string]Conn),
		mutex:      &sync.Mutex{},
		conn2ips:   make(map[string]string),
		ip2actives: make(map[string]time.Time),
		ip2users:   make(map[string]string),
		conn2users: make(map[string]string),
	}
	go cm.CheckTimeout()
	return cm
}

func (cm *ConnMgr) CheckTimeout() {
	for range time.NewTicker(time.Second * CHECK_TIMEOUT_INTEVAL).C {
		timeNow := time.Now()
		iplist := make([]string, 0)
		cm.mutex.Lock()
		for ip, lastActive := range cm.ip2actives {
			if timeNow.Sub(lastActive) > time.Minute*CONNECTION_TIMEOUT {
				iplist = append(iplist, ip)

			}
		}
		cm.mutex.Unlock()

		for _, ip := range iplist {
			cm.RelelaseAddress(ip)
			conn := cm.GetConnByIP(ip)
			if conn != nil {
				cm.DetachIPAddressFromConn(conn)
				cm.DetachUserFromConn(conn)
				conn.Close(false)
			}
		}

	}
}

func (cm *ConnMgr) SetAddressPool(addrespool *AddressPool) {
	cm.addresspool = addrespool
}

func (cm *ConnMgr) AllocAddress() string {

	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if cm.addresspool == nil {
		elog.Error("address pool haven't set")
		return ""
	}
	ip := cm.addresspool.Alloc()
	if ip != "" {
		cm.ip2actives[ip] = time.Now()
	}
	return ip
}

func (cm *ConnMgr) RelelaseAddress(ip string) {

	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if cm.addresspool == nil {
		return
	}
	delete(cm.ip2actives, ip)
	cm.addresspool.Release(ip)
}

func (cm *ConnMgr) IsAllocedAddress(ip string) bool {

	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if cm.addresspool == nil {
		return false
	}
	return cm.addresspool.IsAlloc(ip)
}

func (cm *ConnMgr) UpdateConnActiveTime(conn Conn) {

	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	ip, ok := cm.conn2ips[conn.String()]
	if ok {
		cm.ip2actives[ip] = time.Now()
	}
}

func (cm *ConnMgr) AttachIPAddressToConn(ip string, conn Conn) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	sconn, ok := cm.ip2conns[ip]
	if ok {
		delete(cm.conn2ips, sconn.String())
	}
	cm.ip2conns[ip] = conn
	cm.conn2ips[conn.String()] = ip
}

func (cm *ConnMgr) IsDetached(ip string) bool {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	_, ok := cm.ip2conns[ip]
	return ok
}

func (cm *ConnMgr) DetachIPAddressFromConn(conn Conn) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	ip, ok := cm.conn2ips[conn.String()]
	if ok {
		sconn, ok := cm.ip2conns[ip]
		if ok && sconn.String() == conn.String() {
			delete(cm.ip2conns, ip)
		}
		delete(cm.conn2ips, conn.String())
	}
}

func (cm *ConnMgr) AttachUserToConn(user string, conn Conn) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	cm.conn2users[conn.String()] = user
}

func (cm *ConnMgr) DetachUserFromConn(conn Conn) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	delete(cm.conn2users, conn.String())
}

func (cm *ConnMgr) GetConnAttachUser(conn Conn) string {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	return cm.conn2users[conn.String()]
}

func (cm *ConnMgr) AttachUserToIP(user string, ip string) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	cm.ip2users[ip] = user
}

func (cm *ConnMgr) DetachUserFromIP(ip string) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	delete(cm.ip2users, ip)
}

func (cm *ConnMgr) GetIPAttachUser(ip string) string {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	return cm.ip2users[ip]
}

func (cm *ConnMgr) GetConnByIP(ip string) Conn {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	return cm.ip2conns[ip]
}

func (cm *ConnMgr) GeIPByConn(conn Conn) string {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	return cm.conn2ips[conn.String()]
}

func (cm *ConnMgr) SetConn(streamId string, conn Conn) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	cm.conns[streamId] = conn
}

func (cm *ConnMgr) GetConn(streamId string) Conn {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	return cm.conns[streamId]
}

func (cm *ConnMgr) RemoveConn(streamId string) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	delete(cm.conns, streamId)
}
