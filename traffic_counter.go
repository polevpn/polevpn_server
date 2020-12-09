package main

import (
	"time"
)

const (
	TRAFFIC_LIMIT_INTERVAL = 10
)

type TrafficCounter struct {
	upIPbytes        uint64
	downIPbytes      uint64
	upIPbytesTotal   uint64
	downIPbytesTotal uint64
	upIPLastTime     time.Time
	downIPLastTime   time.Time
}

func NewTrafficCounter() *TrafficCounter {
	return &TrafficCounter{
		upIPbytes:        0,
		downIPbytes:      0,
		upIPLastTime:     time.Now(),
		downIPLastTime:   time.Now(),
		upIPbytesTotal:   0,
		downIPbytesTotal: 0,
	}
}

func (tl *TrafficCounter) UPStreamCount(bytes uint64) (uint64, time.Time) {

	now := time.Now()

	if now.Sub(tl.upIPLastTime) > time.Millisecond*TRAFFIC_LIMIT_INTERVAL {
		tl.upIPbytes = 0
		tl.upIPLastTime = time.Now()
	}

	tl.upIPbytes += bytes
	tl.upIPbytesTotal += bytes
	return tl.upIPbytes, tl.upIPLastTime
}

func (tl *TrafficCounter) DownStreamCount(bytes uint64) (uint64, time.Time) {
	now := time.Now()

	if now.Sub(tl.downIPLastTime) > time.Millisecond*TRAFFIC_LIMIT_INTERVAL {
		tl.downIPbytes = 0
		tl.downIPLastTime = time.Now()
	}

	tl.downIPbytes += bytes
	tl.downIPbytesTotal += bytes
	return tl.downIPbytes, tl.downIPLastTime

}

func (tl *TrafficCounter) DownStreamTotalBytes() uint64 {

	return tl.downIPbytesTotal
}

func (tl *TrafficCounter) UPStreamTotalBytes() uint64 {

	return tl.upIPbytesTotal
}
