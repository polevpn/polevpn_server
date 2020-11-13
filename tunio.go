package main

import (
	"errors"
	"io"
	"os/exec"

	"github.com/polevpn/elog"
	"github.com/songgao/water"
)

type TunIO struct {
	ifce    *water.Interface
	wch     chan []byte
	mtu     int
	handler *PacketDispatcher
	closed  bool
}

func NewTunIO(size int, handler *PacketDispatcher) (*TunIO, error) {

	config := water.Config{
		DeviceType: water.TUN,
	}
	ifce, err := water.New(config)
	if err != nil {
		return nil, err
	}

	return &TunIO{ifce: ifce, wch: make(chan []byte, size), mtu: 1500, handler: handler, closed: false}, nil
}

// ip addr add dev tun0 local 10.8.0.1 peer 10.8.0.2
// ip route add 10.8.0.0/24 via 10.8.0.2
func (t *TunIO) SetIPAddress(ip1 string, ip2 string) error {
	var err error
	_, err = exec.Command("bash", "-c", "ip addr add dev "+t.ifce.Name()+" local "+ip1+" peer "+ip2).Output()

	if err != nil {
		return err
	}
	return nil
}

func (t *TunIO) Enanble() error {

	var err error
	_, err = exec.Command("bash", "-c", "ip link set "+t.ifce.Name()+" up").Output()

	if err != nil {
		return err
	}
	return nil
}

func (t *TunIO) AddRoute(cidr string, gw string) error {
	var err error
	_, err = exec.Command("bash", "-c", "ip route add "+cidr+" via "+gw).Output()

	if err != nil {
		return err
	}

	return err

}

func (t *TunIO) Close() error {
	if t.closed == true {
		return nil
	}

	if t.wch != nil {
		t.wch <- nil
		close(t.wch)
	}
	t.closed = true
	return t.ifce.Close()
}

func (t *TunIO) Read() {
	defer func() {
		t.Close()
	}()

	defer PanicHandler()

	for {

		pkt := make([]byte, t.mtu)
		n, err := t.ifce.Read(pkt)
		if err != nil {
			elog.Error("read pkg from tun fail", err)
		}
		pkt = pkt[:n]
		t.handler.Dispatch(pkt)
	}

}

func (t *TunIO) Write() {
	defer PanicHandler()
	for {
		select {
		case pkt, ok := <-t.wch:
			if !ok {
				elog.Error("get pkt from write channel fail,maybe channel closed")
				return
			} else {
				if pkt == nil {
					elog.Info("exit write process")
					return
				}
				_, err := t.ifce.Write(pkt)
				if err != nil {
					if err == io.EOF {
						elog.Info("tun may be closed")
					} else {
						elog.Error("tun write error", err)
					}
					return
				}
			}
		}
	}
}

func (t *TunIO) Enqueue(pkt []byte) error {
	if t.wch == nil {
		return errors.New("write channel is nil")
	}
	t.wch <- pkt
	return nil
}
