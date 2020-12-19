package main

import (
	"encoding/binary"
	"io"
	"math/rand"
	"sync"
	"time"

	"github.com/polevpn/elog"
	"github.com/polevpn/h2conn"
	"github.com/polevpn/netstack/tcpip/header"
	"github.com/polevpn/netstack/tcpip/transport/tcp"
	"github.com/polevpn/netstack/tcpip/transport/udp"
)

const (
	CH_HTTP2_WRITE_SIZE = 2000
)

type Http2Conn struct {
	wg           *sync.WaitGroup
	conn         *h2conn.Conn
	wch          chan []byte
	closed       bool
	handler      *RequestHandler
	downlimit    uint64
	uplimit      uint64
	tcDownStream *TrafficCounter
	tcUpStream   *TrafficCounter
}

func NewHttp2Conn(wg *sync.WaitGroup, conn *h2conn.Conn, downlimit uint64, uplimit uint64, handler *RequestHandler) *Http2Conn {
	return &Http2Conn{
		wg:           wg,
		conn:         conn,
		closed:       false,
		wch:          make(chan []byte, CH_HTTP2_WRITE_SIZE),
		handler:      handler,
		downlimit:    downlimit,
		uplimit:      uplimit,
		tcDownStream: NewTrafficCounter(TRAFFIC_LIMIT_INTERVAL * time.Millisecond),
		tcUpStream:   NewTrafficCounter(TRAFFIC_LIMIT_INTERVAL * time.Millisecond),
	}
}

func (h2c *Http2Conn) Close(flag bool) error {
	if h2c.closed == false {
		h2c.closed = true
		if h2c.wch != nil {
			h2c.wch <- nil
			close(h2c.wch)
		}
		err := h2c.conn.Close()
		if flag {
			go h2c.handler.OnClosed(h2c, false)
		}
		return err
	}
	return nil
}

func (h2c *Http2Conn) String() string {
	return h2c.conn.RemoteAddr().String() + "->" + h2c.conn.LocalAddr().String()
}

func (h2c *Http2Conn) IsClosed() bool {
	return h2c.closed
}

func (h2c *Http2Conn) checkStreamLimit(pkt []byte, tfcounter *TrafficCounter, limit uint64) (bool, time.Duration) {
	bytes, ltime := tfcounter.StreamCount(uint64(len(pkt)))
	if bytes > limit/(1000/uint64(tfcounter.StreamCountInterval()/time.Millisecond)) {
		duration := ltime.Add(tfcounter.StreamCountInterval()).Sub(time.Now())
		if duration > 0 {
			drop := false
			if len(h2c.wch) > CH_WEBSOCKET_WRITE_SIZE*0.5 {
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

func (h2c *Http2Conn) Read() {
	defer func() {
		h2c.Close(true)
		h2c.wg.Done()
	}()

	defer PanicHandler()

	for {
		var preOffset = 0
		prefetch := make([]byte, 2)

		for {
			n, err := h2c.conn.Read(prefetch[preOffset:])
			if err != nil {
				if err == io.ErrUnexpectedEOF || err == io.EOF {
					elog.Info(h2c.String(), "conn closed")
				} else {
					elog.Error(h2c.String(), "conn read exception:", err)
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
			n, err := h2c.conn.Read(pkt[offset:])
			if err != nil {
				if err == io.ErrUnexpectedEOF || err == io.EOF {
					elog.Info(h2c.String(), "conn closed")
				} else {
					elog.Error(h2c.String(), "conn read exception:", err)
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
			limit, duration := h2c.checkStreamLimit(ppkt.Payload(), h2c.tcUpStream, h2c.uplimit)
			if limit {
				if duration > 0 {
					time.Sleep(duration)
				} else {
					continue
				}
			}
		}
		h2c.handler.OnRequest(pkt, h2c)

	}

}

func (h2c *Http2Conn) Write() {

	defer h2c.wg.Done()
	defer PanicHandler()

	for {

		pkt, ok := <-h2c.wch
		if !ok {
			elog.Error(h2c.String(), "get pkt from write channel fail,maybe channel closed")
			return
		}
		if pkt == nil {
			elog.Info(h2c.String(), "exit write process")
			return
		}

		ppkt := PolePacket(pkt)
		if ppkt.Cmd() == CMD_S2C_IPDATA {
			//traffic limit
			limit, duration := h2c.checkStreamLimit(ppkt.Payload(), h2c.tcDownStream, h2c.downlimit)
			if limit {
				if duration > 0 {
					time.Sleep(duration)
				} else {
					continue
				}
			}
		}
		_, err := h2c.conn.Write(pkt)
		if err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				elog.Info(h2c.String(), "conn closed")
			} else {
				elog.Error(h2c.String(), "conn write exception:", err)
			}
			return
		}
	}
}

func (h2c *Http2Conn) Send(pkt []byte) {
	if h2c.closed == true {
		return
	}
	if h2c.wch != nil {
		h2c.wch <- pkt
	}
}
