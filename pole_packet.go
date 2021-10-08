package main

import "encoding/binary"

const (
	CMD_ALLOC_IPADDR  = 0x1
	CMD_S2C_IPDATA    = 0x2
	CMD_C2S_IPDATA    = 0x3
	CMD_HEART_BEAT    = 0x4
	CMD_CLIENT_CLOSED = 0x5
	CMD_KICK_OUT      = 0x6
	CMD_USER_AUTH     = 0x7
)

const (
	POLE_PACKET_HEADER_LEN = 4
)

type PolePacket []byte

func (p PolePacket) Len() uint16 {
	return binary.BigEndian.Uint16(p[0:2])
}

func (p PolePacket) Cmd() uint16 {
	return binary.BigEndian.Uint16(p[2:4])
}

func (p PolePacket) Payload() []byte {
	return p[POLE_PACKET_HEADER_LEN:]
}

func (p PolePacket) SetLen(len uint16) {
	binary.BigEndian.PutUint16(p[0:], len)
}

func (p PolePacket) SetCmd(cmd uint16) {
	binary.BigEndian.PutUint16(p[2:], cmd)
}
