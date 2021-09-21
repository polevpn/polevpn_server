package main

import (
	"net"
	"sync"
)

type AddressPool struct {
	pool     map[string]bool
	bindips  map[string]string
	rbindips map[string]string
	mutex    *sync.Mutex
	gw       string
	network  *net.IPNet
}

func NewAddressPool(cidr string, bindips map[string]string) (*AddressPool, error) {

	rbindips := make(map[string]string)

	for user, ip := range bindips {
		rbindips[ip] = user
	}

	_, network, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, err
	}
	networkipv4 := network.IP.To4()
	start := net.IPv4(networkipv4[0], networkipv4[1], networkipv4[2], networkipv4[3]).To4()
	var gw string
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
			gw = start.String()
			continue
		}
		_, ok := rbindips[start.String()]
		if ok {
			pool[start.String()] = true
		} else {
			pool[start.String()] = false
		}

	}
	return &AddressPool{pool: pool, mutex: &sync.Mutex{}, gw: gw, network: network, bindips: bindips, rbindips: rbindips}, nil
}

func (ap *AddressPool) Alloc() string {

	for ip, used := range ap.pool {
		if !used {
			ap.pool[ip] = true
			return ip
		}
	}
	return ""
}

func (ap *AddressPool) GatewayIP() string {
	return ap.gw
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

func (ap *AddressPool) GetBindIP(user string) string {
	return ap.bindips[user]
}

func (ap *AddressPool) GetBindUser(ip string) string {
	return ap.rbindips[ip]
}

func (ap *AddressPool) IsAlloc(ip string) bool {

	v, ok := ap.pool[ip]
	if ok {
		return v
	}
	return ok
}
