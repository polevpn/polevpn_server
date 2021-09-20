package main

import (
	"encoding/binary"
	"io"
	"math/rand"
	"sync"
	"time"

	"github.com/polevpn/elog"
	"github.com/polevpn/h3conn"
	"github.com/polevpn/netstack/tcpip/header"
	"github.com/polevpn/netstack/tcpip/transport/tcp"
	"github.com/polevpn/netstack/tcpip/transport/udp"
)

const (
	CH_h3cP_WRITE_SIZE = 2000
)

type Http3Conn struct {
	wg           *sync.WaitGroup
	conn         *h3conn.Conn
	wch          chan []byte
	closed       bool
	handler      *RequestHandler
	downlimit    uint64
	uplimit      uint64
	tcDownStream *TrafficCounter
	tcUpStream   *TrafficCounter
}

func NewHttp3Conn(wg *sync.WaitGroup, conn *h3conn.Conn, downlimit uint64, uplimit uint64, handler *RequestHandler) *Http3Conn {
	return &Http3Conn{
		wg:           wg,
		conn:         conn,
		closed:       false,
		wch:          make(chan []byte, CH_h3cP_WRITE_SIZE),
		handler:      handler,
		downlimit:    downlimit,
		uplimit:      uplimit,
		tcDownStream: NewTrafficCounter(TRAFFIC_LIMIT_INTERVAL * time.Millisecond),
		tcUpStream:   NewTrafficCounter(TRAFFIC_LIMIT_INTERVAL * time.Millisecond),
	}
}

func (h3c *Http3Conn) Close(flag bool) error {
	if h3c.closed == false {
		h3c.closed = true
		if h3c.wch != nil {
			h3c.wch <- nil
			close(h3c.wch)
		}
		err := h3c.conn.Close()
		if flag {
			go h3c.handler.OnClosed(h3c, false)
		}
		return err
	}
	return nil
}

func (h3c *Http3Conn) String() string {
	return h3c.conn.RemoteAddr().String() + "->" + h3c.conn.LocalAddr().String()
}

func (h3c *Http3Conn) IsClosed() bool {
	return h3c.closed
}

func (h3c *Http3Conn) checkStreamLimit(pkt []byte, tfcounter *TrafficCounter, limit uint64) (bool, time.Duration) {
	bytes, ltime := tfcounter.StreamCount(uint64(len(pkt)))
	if bytes > limit/(1000/uint64(tfcounter.StreamCountInterval()/time.Millisecond)) {
		duration := ltime.Add(tfcounter.StreamCountInterval()).Sub(time.Now())
		if duration > 0 {
			drop := false
			if len(h3c.wch) > CH_WEBSOCKET_WRITE_SIZE*0.5 {
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

func (h3c *Http3Conn) Read() {

	defer func() {
		h3c.wg.Done()
		h3c.Close(true)
	}()

	defer PanicHandler()

	for {
		var preOffset = 0
		prefetch := make([]byte, 2)

		for {
			n, err := h3c.conn.Read(prefetch[preOffset:])
			if err != nil {
				if err == io.ErrUnexpectedEOF || err == io.EOF {
					elog.Info(h3c.String(), "conn closed")
				} else {
					elog.Error(h3c.String(), "conn read exception:", err)
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
			n, err := h3c.conn.Read(pkt[offset:])
			if err != nil {
				if err == io.ErrUnexpectedEOF || err == io.EOF {
					elog.Info(h3c.String(), "conn closed")
				} else {
					elog.Error(h3c.String(), "conn read exception:", err)
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
			limit, duration := h3c.checkStreamLimit(ppkt.Payload(), h3c.tcUpStream, h3c.uplimit)
			if limit {
				if duration > 0 {
					time.Sleep(duration)
				} else {
					continue
				}
			}
		}
		h3c.handler.OnRequest(pkt, h3c)

	}

}

func (h3c *Http3Conn) Write() {

	defer h3c.wg.Done()
	defer PanicHandler()

	for {

		pkt, ok := <-h3c.wch
		if !ok {
			elog.Error(h3c.String(), "get pkt from write channel fail,maybe channel closed")
			return
		}
		if pkt == nil {
			elog.Info(h3c.String(), "exit write process")
			return
		}

		ppkt := PolePacket(pkt)
		if ppkt.Cmd() == CMD_S2C_IPDATA {
			//traffic limit
			limit, duration := h3c.checkStreamLimit(ppkt.Payload(), h3c.tcDownStream, h3c.downlimit)
			if limit {
				if duration > 0 {
					time.Sleep(duration)
				} else {
					continue
				}
			}
		}
		_, err := h3c.conn.Write(pkt)
		if err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				elog.Info(h3c.String(), "conn closed")
			} else {
				elog.Error(h3c.String(), "conn write exception:", err)
			}
			return
		}
	}
}

func (h3c *Http3Conn) Send(pkt []byte) {
	if h3c.closed == true {
		return
	}
	if h3c.wch != nil {
		h3c.wch <- pkt
	}
}
