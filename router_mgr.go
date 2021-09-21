package main

import (
	"net"
	"sync"
)

type RouterMgr struct {
	routetable map[string]string
	mutex      *sync.RWMutex
}

func NewRouterMgr() *RouterMgr {
	rm := &RouterMgr{
		routetable: make(map[string]string),
		mutex:      &sync.RWMutex{},
	}
	return rm
}

func (rm *RouterMgr) AddRoute(cidr string, gw string) bool {

	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	_, ok := rm.routetable[cidr]
	if ok {
		return false
	}

	rm.routetable[cidr] = gw

	return true
}

func (rm *RouterMgr) GetRoute(cidr string) string {

	rm.mutex.RLock()
	defer rm.mutex.RUnlock()

	return rm.routetable[cidr]

}

func (rm *RouterMgr) DelRoute(cidr string) {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	delete(rm.routetable, cidr)
}

func (rm *RouterMgr) FindRoute(destIP string) string {

	rm.mutex.RLock()
	defer rm.mutex.RUnlock()

	var defaultGateway string
	for route, gw := range rm.routetable {

		_, subnet, err := net.ParseCIDR(route)

		if err != nil {
			continue
		}
		find := subnet.Contains(net.IP(destIP))
		if route == "0.0.0.0/0" {
			defaultGateway = gw
		} else if find && (route != "0.0.0.0/0") {
			return gw
		}
	}
	return defaultGateway
}
