// Copyright 2012 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"syscall"
)

const (
	Version      = 4  // protocol version
	HeaderLen    = 20 // header length without extension headers
	maxHeaderLen = 60 // sensible default, revisit if later RFCs define new usage of version and header length fields
)

var (
	ErrMissingAddress           = errors.New("missing address")
	ErrMissingHeader            = errors.New("missing header")
	ErrHeaderTooShort           = errors.New("header too short")
	ErrBufferTooShort           = errors.New("buffer too short")
	ErrInvalidConnType          = errors.New("invalid conn type")
	ErrOpNoSupport              = errors.New("operation not supported")
	ErrNoSuchInterface          = errors.New("no such interface")
	ErrNoSuchMulticastInterface = errors.New("no such multicast interface")
)

type HeaderFlags int

const (
	MoreFragments HeaderFlags = 1 << iota // more fragments flag
	DontFragment                          // don't fragment flag
)

func NewIPv4Header() *Header {
	return &Header{}
}

// A Header represents an IPv4 header.
type Header struct {
	Version  int         // protocol version
	Len      int         // header length
	TOS      int         // type-of-service
	TotalLen int         // packet total length
	ID       int         // identification
	Flags    HeaderFlags // flags
	FragOff  int         // fragment offset
	TTL      int         // time-to-live
	Protocol int         // next protocol
	Checksum uint16      // checksum
	Src      net.IP      // source address
	Dst      net.IP      // destination address
	Options  []byte      // options, extension headers

	Payload []byte
}

func (h *Header) WithSource(v net.IP) *Header {
	h.Src = v
	return h
}

func (h *Header) WithCSum(csum uint16) *Header {
	h.Checksum = csum
	return h
}

func (h *Header) WithDestination(v net.IP) *Header {
	h.Dst = v
	return h
}

func (h *Header) WithID(id int) *Header {
	h.ID = id
	return h
}

func (h *Header) String() string {
	if h == nil {
		return "<nil>"
	}
	return fmt.Sprintf("ver=%d hdrlen=%d tos=%#x totallen=%d id=%#x flags=%#x fragoff=%#x ttl=%d proto=%d cksum=%#x src=%v dst=%v", h.Version, h.Len, h.TOS, h.TotalLen, h.ID, h.Flags, h.FragOff, h.TTL, h.Protocol, h.Checksum, h.Src, h.Dst)
}

func New() *Header {
	return &Header{
		Version:  4,
		Len:      20,
		TOS:      0,
		TotalLen: 52,
		Flags:    2,
		TTL:      128,
		Protocol: 6,
		Src:      nil,
		Dst:      nil,
		Options:  []byte{},
		ID:       0,
	}
}

// Marshal returns the binary encoding of the IPv4 header h.
func (h *Header) Marshal() ([]byte, error) {
	if h == nil {
		return nil, syscall.EINVAL
	}
	if h.Len < HeaderLen {
		return nil, ErrHeaderTooShort
	}

	h.TotalLen = 20 + len(h.Payload)

	hdrlen := HeaderLen + len(h.Options)

	b := make([]byte, hdrlen+len(h.Payload))
	b[0] = byte(Version<<4 | (hdrlen >> 2 & 0x0f))
	b[1] = byte(h.TOS)
	flagsAndFragOff := (h.FragOff & 0x1fff) | int(h.Flags<<13)
	binary.BigEndian.PutUint16(b[2:4], uint16(h.TotalLen))
	binary.BigEndian.PutUint16(b[4:6], uint16(h.ID))
	binary.BigEndian.PutUint16(b[6:8], uint16(flagsAndFragOff))
	b[8] = byte(h.TTL)
	b[9] = byte(h.Protocol)
	if h.Checksum != 0 {
		binary.BigEndian.PutUint16(b[10:12], uint16(h.Checksum))
	}
	if ip := h.Src.To4(); ip != nil {
		copy(b[12:16], ip[:net.IPv4len])
	}
	if ip := h.Dst.To4(); ip != nil {
		copy(b[16:20], ip[:net.IPv4len])
	} else {
		return nil, ErrMissingAddress
	}
	if len(h.Options) > 0 {
		copy(b[HeaderLen:], h.Options)
	}

	if h.Checksum == 0 {
		h.Checksum = CheckSum(b[:20])
		binary.BigEndian.PutUint16(b[10:12], uint16(h.Checksum))
	}

	copy(b[hdrlen:], h.Payload)

	return b, nil
}

// ParseHeader parses b as an IPv4 header.
func Parse(b []byte) (*Header, error) {
	h := &Header{}
	return h, h.Unmarshal(b)
}

func (h *Header) Unmarshal(b []byte) error {
	if len(b) < HeaderLen {
		return ErrHeaderTooShort
	}
	hdrlen := int(b[0]&0x0f) << 2
	if hdrlen > len(b) {
		return ErrBufferTooShort
	}

	h.Version = int(b[0] >> 4)
	h.Len = hdrlen
	h.TOS = int(b[1])
	h.ID = int(binary.BigEndian.Uint16(b[4:6]))
	h.TTL = int(b[8])
	h.Protocol = int(b[9])
	h.Checksum = binary.BigEndian.Uint16(b[10:12])
	h.Src = net.IPv4(b[12], b[13], b[14], b[15])
	h.Dst = net.IPv4(b[16], b[17], b[18], b[19])
	h.TotalLen = int(binary.BigEndian.Uint16(b[2:4]))
	h.FragOff = int(binary.BigEndian.Uint16(b[6:8]))

	h.Flags = HeaderFlags(h.FragOff&0xe000) >> 13
	h.FragOff = h.FragOff & 0x1fff
	if hdrlen-HeaderLen > 0 {
		h.Options = make([]byte, hdrlen-HeaderLen)
		copy(h.Options, b[HeaderLen:])
	}

	h.Payload = b[20:h.TotalLen]

	return nil
}
