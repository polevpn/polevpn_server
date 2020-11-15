package main

import "testing"

func TestBitOp(t *testing.T) {
	b := make([]byte, 1)
	b[0] = 0x60
	var a byte = 0x60
	a = a >> 4
	t.Log(a)
	c := b[0]
	c = c >> 4
	t.Log(c, b[0])
}
