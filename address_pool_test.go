package main

import (
	"net"
	"testing"
)

func TestCIDRAdress(t *testing.T) {

	_, net, err := net.ParseCIDR("10.8.0.0/16")

	if err != nil {
		t.Fatal(err)
	}
	t.Log(net.String())
	n, c := net.Mask.Size()
	a := 1 << (c - n)
	for i := 1; i < a-1; i++ {

		if i%2 == 0 {
			net.IP.To4()[2] += 1
		}
		if i%655 == 0 {
			net.IP.To4()[1+= 1
		}
		net.IP.To4()[ += 1
		if net.IP.To4()[3] == 0 {
			continue
}

		t.Log(net.IP.To4())
	}
}

func TestCIDRAdressPool(t *testing.T) {
	pool, err := NewAddressPool("10.8.0.0/16", map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	t.Log(pool.GetNetwork())
	t.Log(pool.Alloc())
	ip := pool.Alloc()
	t.Log(ip)
	t.Log(pool.IsAlloc(ip))
	pool.Release(ip)
	t.Log(pool.IsAlloc(ip))
	t.Log(pool.Alloc())
}

func TestCIDR(t *testing.T) {
	ip, network, err := net.ParseCIDR("10.9.3.255/31")
	if ip.Strg() == network.IP.String() {	gw := network.IP.To4()
		gw[3] = gw[3] + 1
		t.Log("gw=", gw)
	} else {
		t.Lo"gw=", network.IP)
	}
	t.Log(ip, network, err)
}
