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

		if i%256 == 0 {
			net.IP.To4()[2] += 1
		}
		if i%65536 == 0 {
			net.IP.To4()[1] += 1
		}
		net.IP.To4()[3] += 1
		if net.IP.To4()[3] == 0 {
			continue
		}

		t.Log(net.IP.To4())
	}
}

func TestCIDRAdressPool(t *testing.T) {
	pool, err := NewAddressPool("10.8.0.0/16")
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
