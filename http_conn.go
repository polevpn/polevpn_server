package main

import (
	"encoding/binary"
	"io"
	"math/rand"
	"net/http"
	"time"

	"github.com/polevpn/elog"
	"github.com/polevpn/netstack/tcpip/header"
	"github.com/polevpn/netstack/tcpip/transport/tcp"
	"github.com/polevpn/netstack/tcpip/transport/udp"
)

const (
	CH_HTTP_WRITE_SIZE = 2000
)

type HttpConn struct {
	stream       string
	up           io.ReadCloser
	down         io.Writer
	flusher      http.Flusher
	wch          chan []byte
	closed       bool
	handler      *RequestHandler
	downlimit    uint64
	uplimit      uint64
	tcDownStream *TrafficCounter
	tcUpStream   *TrafficCounter
}

func NewHttpConn(stream string, downlimit uint64, uplimit uint64, handler *RequestHandler) *HttpConn {
	return &HttpConn{
		stream:       stream,
		closed:       false,
		wch:          make(chan []byte, CH_HTTP2_WRITE_SIZE),
		handler:      handler,
		downlimit:    downlimit,
		uplimit:      uplimit,
		tcDownStream: NewTrafficCounter(TRAFFIC_LIMIT_INTERVAL * time.Millisecond),
		tcUpStream:   NewTrafficCounter(TRAFFIC_LIMIT_INTERVAL * time.Millisecond),
	}
}

func (hc *HttpConn) SetUpStream(up io.ReadCloser) {
	hc.up = up
}

func (hc *HttpConn) SetDownStream(down io.Writer, flusher http.Flusher) {
	hc.down = down
	hc.flusher = flusher
}

func (hc *HttpConn) Ready() bool {
	return hc.up != nil && hc.down != nil
}

func (hc *HttpConn) Close(flag bool) error {
	if hc.closed == false {
		hc.closed = true
		if hc.wch != nil {
			hc.wch <- nil
			close(hc.wch)
		}
		err := hc.up.Close()
		if flag {
			go hc.handler.OnClosed(hc, false)
		}
		return err
	}
	return nil
}

func (hc *HttpConn) String() string {
	return hc.stream
}

func (hc *HttpConn) IsClosed() bool {
	return hc.closed
}

func (hc *HttpConn) checkStreamLimit(pkt []byte, tfcounter *TrafficCounter, limit uint64) (bool, time.Duration) {
	bytes, ltime := tfcounter.StreamCount(uint64(len(pkt)))
	if bytes > limit/(1000/uint64(tfcounter.StreamCountInterval()/time.Millisecond)) {
		duration := ltime.Add(tfcounter.StreamCountInterval()).Sub(time.Now())
		if duration > 0 {
			drop := false
			if len(hc.wch) > CH_WEBSOCKET_WRITE_SIZE*0.5 {
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

func (hc *HttpConn) Read() {
	defer func() {
		hc.Close(true)
	}()

	defer PanicHandler()

	for {
		prefetch := make([]byte, 2)
		_, err := hc.up.Read(prefetch)
		if err != nil {
			if err == io.ErrUnexpectedEOF || err == io.EOF {
				elog.Info(hc.String(), "conn closed")
			} else {
				elog.Error(hc.String(), "conn read exception:", err)
			}
			return
		}

		len := binary.BigEndian.Uint16(prefetch)

		pkt := make([]byte, len)
		copy(pkt, prefetch)
		var offset uint16 = 2
		for {
			n, err := hc.up.Read(pkt[offset:])
			if err != nil {
				if err == io.ErrUnexpectedEOF || err == io.EOF {
					elog.Info(hc.String(), "conn closed")
				} else {
					elog.Error(hc.String(), "conn read exception:", err)
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
			limit, duration := hc.checkStreamLimit(ppkt.Payload(), hc.tcUpStream, hc.uplimit)
			if limit {
				if duration > 0 {
					time.Sleep(duration)
				} else {
					continue
				}
			}
		}
		hc.handler.OnRequest(pkt, hc)

	}

}

func (hc *HttpConn) Write() {

	defer PanicHandler()

	for {

		pkt, ok := <-hc.wch
		if !ok {
			elog.Error(hc.String(), "get pkt from write channel fail,maybe channel closed")
			return
		}
		if pkt == nil {
			elog.Info(hc.String(), "exit write process")
			return
		}

		ppkt := PolePacket(pkt)
		if ppkt.Cmd() == CMD_S2C_IPDATA {
			//traffic limit
			limit, duration := hc.checkStreamLimit(ppkt.Payload(), hc.tcDownStream, hc.downlimit)
			if limit {
				if duration > 0 {
					time.Sleep(duration)
				} else {
					continue
				}
			}
		}
		_, err := hc.down.Write(pkt)
		if err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				elog.Info(hc.String(), "conn closed")
			} else {
				elog.Error(hc.String(), "conn write exception:", err)
			}
			return
		}
		hc.flusher.Flush()
	}
}

func (hc *HttpConn) Send(pkt []byte) {
	if hc.closed == true {
		return
	}
	if hc.wch != nil {
		hc.wch <- pkt
	}
}
