package main

import (
	"net"
	"sort"
	"sync"
)

type RouterMgr struct {
	routetable  map[string]string
	nettable    map[string]*net.IPNet
	mutex       *sync.RWMutex
	sortedtable []string
}

func NewRouterMgr() *RouterMgr {
	rm := &RouterMgr{
		routetable:  make(map[string]string),
		mutex:       &sync.RWMutex{},
		sortedtable: make([]string, 0),
		nettable:    make(map[string]*net.IPNet),
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

	var sortedtable []string
	for route := range rm.routetable {
		sortedtable = append(sortedtable, route)
		_, subnet, err := net.ParseCIDR(route)
		if err != nil {
			continue
		}
		rm.nettable[route] = subnet
	}

	sort.Strings(sortedtable)
	rm.sortedtable = sortedtable

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

	var sortedtable []string
	for route := range rm.routetable {
		sortedtable = append(sortedtable, route)
		_, subnet, err := net.ParseCIDR(route)
		if err != nil {
			continue
		}
		rm.nettable[route] = subnet
	}

	sort.Strings(sortedtable)
	rm.sortedtable = sortedtable

}

func (rm *RouterMgr) FindRoute(destIP net.IP) string {

	rm.mutex.RLock()
	defer rm.mutex.RUnlock()
	slen := len(rm.sortedtable)

	for i := slen - 1; i > -1; i-- {

		route := rm.sortedtable[i]
		subnet := rm.nettable[route]
		if subnet == nil {
			continue
		}
		gw := rm.routetable[route]
		find := subnet.Contains(net.IP(destIP))
		if find {
			return gw
		}
	}
	return ""
}
