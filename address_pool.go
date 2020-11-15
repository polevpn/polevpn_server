package main

import (
	"net"
	"sync"
)

type AddressPool struct {
	pool    map[string]bool
	mutex   *sync.Mutex
	gw1     string
	gw2     string
	network *net.IPNet
}

func NewAddressPool(cidr string) (*AddressPool, error) {

	_, network, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, err
	}
	networkipv4 := network.IP.To4()
	start := net.IPv4(networkipv4[0], networkipv4[1], networkipv4[2], networkipv4[3]).To4()
	var gw1, gw2 string
	pool := make(map[string]bool)

	n, c := network.Mask.Size()
	a := 1 << (c - n)
	for i := 1; i < a-1; i++ {

		if i%256 == 0 {
			start[2] += 1
		}
		if i%65536 == 0 {
			start[1] += 1
		}
		start[3] += 1
		if start[3] == 0 {
			continue
		}
		if i == 1 {
			gw1 = start.String()
			continue
		}
		if i == 2 {
			gw2 = start.String()
			continue
		}
		pool[start.String()] = false
	}
	return &AddressPool{pool: pool, mutex: &sync.Mutex{}, gw1: gw1, gw2: gw2, network: network}, nil
}

func (ap *AddressPool) Alloc() string {

	for ip, used := range ap.pool {
		if used == false {
			ap.pool[ip] = true
			return ip
		}
	}
	return ""
}

func (ap *AddressPool) GatewayIP1() string {
	return ap.gw1
}
func (ap *AddressPool) GatewayIP2() string {

	return ap.gw2
}

func (ap *AddressPool) GetNetwork() string {
	return ap.network.String()
}

func (ap *AddressPool) Release(ip string) {
	_, ok := ap.pool[ip]
	if ok {
		ap.pool[ip] = false
	}
}

func (ap *AddressPool) IsAlloc(ip string) bool {

	v, ok := ap.pool[ip]
	if ok {
		return v
	}
	return ok
}
