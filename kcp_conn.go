package main

import (
	"encoding/binary"
	"io"
	"math/rand"
	"time"

	"github.com/polevpn/elog"
	"github.com/polevpn/netstack/tcpip/header"
	"github.com/polevpn/netstack/tcpip/transport/tcp"
	"github.com/polevpn/netstack/tcpip/transport/udp"
	"github.com/xtaci/kcp-go/v5"
)

const (
	CH_KCP_WRITE_SIZE = 2000
)

type KCPConn struct {
	conn         *kcp.UDPSession
	wch          chan []byte
	closed       bool
	handler      *RequestHandler
	downlimit    uint64
	uplimit      uint64
	tcDownStream *TrafficCounter
	tcUpStream   *TrafficCounter
}

func NewKCPConn(conn *kcp.UDPSession, downlimit uint64, uplimit uint64, handler *RequestHandler) *KCPConn {
	return &KCPConn{
		conn:         conn,
		closed:       false,
		wch:          make(chan []byte, CH_KCP_WRITE_SIZE),
		handler:      handler,
		downlimit:    downlimit,
		uplimit:      uplimit,
		tcDownStream: NewTrafficCounter(TRAFFIC_LIMIT_INTERVAL * time.Millisecond),
		tcUpStream:   NewTrafficCounter(TRAFFIC_LIMIT_INTERVAL * time.Millisecond),
	}
}

func (kc *KCPConn) Close(flag bool) error {
	if kc.closed == false {
		kc.closed = true
		if kc.wch != nil {
			kc.wch <- nil
			close(kc.wch)
		}
		err := kc.conn.Close()
		if flag {
			go kc.handler.OnClosed(kc, false)
		}
		return err
	}
	return nil
}

func (kc *KCPConn) String() string {
	return kc.conn.RemoteAddr().String() + "->" + kc.conn.LocalAddr().String()
}

func (kc *KCPConn) IsClosed() bool {
	return kc.closed
}

func (kc *KCPConn) checkStreamLimit(pkt []byte, tfcounter *TrafficCounter, limit uint64) (bool, time.Duration) {
	bytes, ltime := tfcounter.StreamCount(uint64(len(pkt)))
	if bytes > limit/(1000/uint64(tfcounter.StreamCountInterval()/time.Millisecond)) {
		duration := ltime.Add(tfcounter.StreamCountInterval()).Sub(time.Now())
		if duration > 0 {
			drop := false
			if len(kc.wch) > CH_WEBSOCKET_WRITE_SIZE*0.5 {
				ippkt := header.IPv4(pkt)
				if ippkt.Protocol() == uint8(tcp.ProtocolNumber) {
					n := rand.Intn(5)
					if n > 2 {
						drop = true
					}
				} else if ippkt.Protocol() == uint8(udp.ProtocolNumber) {
					udppkt := header.UDP(ippkt.Payload())
					if udppkt.DestinationPort() != 53 && udppkt.SourcePort() != 53 {
						drop = true
					}
				}
			}

			if drop {
				return true, 0
			} else {
				return true, duration
			}
		}
	}
	return false, 0
}

func (kc *KCPConn) Read() {
	defer func() {
		kc.Close(true)
	}()

	defer PanicHandler()

	for {
		var preOffset = 0
		prefetch := make([]byte, 2)

		for {
			n, err := kc.conn.Read(prefetch[preOffset:])
			if err != nil {
				if err == io.ErrUnexpectedEOF || err == io.EOF {
					elog.Info(kc.String(), "conn closed")
				} else {
					elog.Error(kc.String(), "conn read exception:", err)
				}
				return
			}
			preOffset += n
			if preOffset >= 2 {
				break
			}
		}

		len := binary.BigEndian.Uint16(prefetch)

		if len < POLE_PACKET_HEADER_LEN {
			elog.Error("invalid pkt len=", len)
			continue
		}

		pkt := make([]byte, len)
		copy(pkt, prefetch)
		var offset uint16 = 2
		for {
			n, err := kc.conn.Read(pkt[offset:])
			if err != nil {
				if err == io.ErrUnexpectedEOF || err == io.EOF {
					elog.Info(kc.String(), "conn closed")
				} else {
					elog.Error(kc.String(), "conn read exception:", err)
				}
				return
			}
			offset += uint16(n)
			if offset >= len {
				break
			}
		}

		ppkt := PolePacket(pkt)
		if ppkt.Cmd() == CMD_C2S_IPDATA {
			//traffic limit
			limit, duration := kc.checkStreamLimit(ppkt.Payload(), kc.tcUpStream, kc.uplimit)
			if limit {
				if duration > 0 {
					time.Sleep(duration)
				} else {
					continue
				}
			}
		}
		kc.handler.OnRequest(pkt, kc)

	}

}

func (kc *KCPConn) Write() {

	defer PanicHandler()

	for {

		pkt, ok := <-kc.wch
		if !ok {
			elog.Error(kc.String(), "get pkt from write channel fail,maybe channel closed")
			return
		}
		if pkt == nil {
			elog.Info(kc.String(), "exit write process")
			return
		}

		ppkt := PolePacket(pkt)
		if ppkt.Cmd() == CMD_S2C_IPDATA {
			//traffic limit
			limit, duration := kc.checkStreamLimit(ppkt.Payload(), kc.tcDownStream, kc.downlimit)
			if limit {
				if duration > 0 {
					time.Sleep(duration)
				} else {
					continue
				}
			}
		}
		_, err := kc.conn.Write(pkt)
		if err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				elog.Info(kc.String(), "conn closed")
			} else {
				elog.Error(kc.String(), "conn write exception:", err)
			}
			return
		}
	}
}

func (kc *KCPConn) Send(pkt []byte) {
	if kc.closed == true {
		return
	}
	if kc.wch != nil {
		kc.wch <- pkt
	}
}
