package main

import (
	"crypto/sha1"
	"encoding/binary"
	"errors"
	"sync"

	"github.com/polevpn/anyvalue"
	"github.com/polevpn/elog"
	"github.com/xtaci/kcp-go/v5"
	"golang.org/x/crypto/pbkdf2"
)

var KCP_KEY = []byte{0x17, 0xef, 0xad, 0x3b, 0x12, 0xed, 0xfa, 0xc9, 0xd7, 0x54, 0x14, 0x5b, 0x3a, 0x4f, 0xb5, 0xf6}
var KCP_SALT = []byte{0xdc, 0x13, 0x54, 0x3c, 0x18, 0xba, 0xf3, 0xe7, 0xd5, 0x34, 0x79, 0x5c, 0x05, 0x1b, 0xc6, 0xe1}

type KCPServer struct {
	requestHandler *RequestHandler
	connMgr        *ConnMgr
	loginchecker   LoginChecker
	uplimit        uint64
	downlimit      uint64
}

func NewKCPServer(uplimit uint64, downlimit uint64, requestHandler *RequestHandler) *KCPServer {
	return &KCPServer{
		requestHandler: requestHandler,
		uplimit:        uplimit,
		downlimit:      downlimit,
	}
}

func (ks *KCPServer) SetLoginCheckHandler(loginchecker LoginChecker) {
	ks.loginchecker = loginchecker
}

func (ks *KCPServer) Listen(wg *sync.WaitGroup, addr string) {
	defer wg.Done()
	key := pbkdf2.Key(KCP_KEY, KCP_SALT, 1024, 32, sha1.New)
	block, _ := kcp.NewAESBlockCrypt(key)
	if listener, err := kcp.ListenWithOptions(addr, block, 10, 3); err == nil {
		for {
			conn, err := listener.AcceptKCP()
			if err != nil {
				elog.Error(err)
				return
			}
			go ks.handleConn(conn)
		}
	} else {
		elog.Error(err)
	}

}

func (ks *KCPServer) readAuthRequest(conn *kcp.UDPSession) (PolePacket, error) {
	var preOffset = 0
	prefetch := make([]byte, 2)

	for {
		n, err := conn.Read(prefetch[preOffset:])
		if err != nil {
			return nil, err
		}
		preOffset += n
		if preOffset >= 2 {
			break
		}
	}

	length := binary.BigEndian.Uint16(prefetch)
	if length < POLE_PACKET_HEADER_LEN {
		return nil, errors.New("invalid pkt len")
	}

	pkt := make([]byte, length)
	copy(pkt, prefetch)
	var offset uint16 = 2
	for {
		n, err := conn.Read(pkt[offset:])
		if err != nil {
			return nil, err
		}
		offset += uint16(n)
		if offset >= length {
			break
		}
	}
	return PolePacket(pkt), nil
}

func (ks *KCPServer) getAuthResponse(av *anyvalue.AnyValue) (PolePacket, error) {
	body, err := av.EncodeJson()
	if err != nil {
		return nil, err
	}
	buf := make([]byte, POLE_PACKET_HEADER_LEN+len(body))
	copy(buf[POLE_PACKET_HEADER_LEN:], body)
	pkt := PolePacket(buf)
	pkt.SetLen(uint16(len(buf)))
	pkt.SetCmd(CMD_USER_AUTH)
	return pkt, nil
}

func (ks *KCPServer) readAndCheckLogin(conn *kcp.UDPSession) (*anyvalue.AnyValue, error) {

	pkt, err := ks.readAuthRequest(conn)

	if err != nil {
		return nil, err
	}

	if pkt.Cmd() == CMD_USER_AUTH {

		av, err := anyvalue.NewFromJson(pkt.Payload())
		if err != nil {
			return nil, err
		}
		user := av.Get("user").AsStr()
		pwd := av.Get("pwd").AsStr()
		ip := av.Get("ip").AsStr()

		elog.Infof("kcp user:%v,pwd:%v,ip:%v connect", user, pwd, ip)

		ret := ks.loginchecker.CheckLogin(user, pwd)
		rav := anyvalue.New()
		rav.Set("ret", 0)
		if !ret {
			rav.Set("ret", 403)
			rpkt, _ := ks.getAuthResponse(rav)
			conn.Write([]byte(rpkt))
			return nil, errors.New("check auth fail")
		}

		if ip != "" {
			if !ks.requestHandler.connmgr.IsAllocedAddress(ip) {
				elog.Errorf("user:%v,pwd:%v,ip:%v reconnect fail,ip address not alloc to it", user, pwd, ip)
				rav.Set("ret", 400)
				rpkt, _ := ks.getAuthResponse(rav)
				conn.Write([]byte(rpkt))
				return nil, errors.New("ip address not alloc to it")
			}

			if ks.requestHandler.connmgr.GetIPAttachUser(ip) != user {
				elog.Errorf("user:%v,pwd:%v,ip:%v reconnect fail,ip address not belong to the user", user, pwd, ip)
				rav.Set("ret", 400)
				rpkt, _ := ks.getAuthResponse(rav)
				conn.Write([]byte(rpkt))
				return nil, errors.New("ip address not belong to the user")
			}
		}

		rpkt, _ := ks.getAuthResponse(rav)
		conn.Write([]byte(rpkt))

		return av, nil

	} else {
		return nil, errors.New("invalid cmd")
	}
}

func (ks *KCPServer) handleConn(conn *kcp.UDPSession) {

	av, err := ks.readAndCheckLogin(conn)
	if err != nil {
		elog.Error(err)
		conn.Close()
		return
	}
	elog.Info("accpet new kcp conn", conn.RemoteAddr().String())

	kcpconn := NewKCPConn(conn, ks.downlimit, ks.uplimit, ks.requestHandler)
	if ks.requestHandler != nil {
		ks.requestHandler.OnConnection(kcpconn, av.Get("user").AsStr(), av.Get("ip").AsStr())
	}
	go kcpconn.Read()
	go kcpconn.Write()
}
