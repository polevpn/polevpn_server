package main

import (
	"time"
)

type TrafficCounter struct {
	IPbytes         uint64
	IPbytesTotal    uint64
	IPLastTime      time.Time
	IPCountInterval time.Duration
}

func NewTrafficCounter(interval time.Duration) *TrafficCounter {
	return &TrafficCounter{
		IPbytes:         0,
		IPLastTime:      time.Now(),
		IPbytesTotal:    0,
		IPCountInterval: interval,
	}
}

func (tl *TrafficCounter) StreamCount(bytes uint64) (uint64, time.Time) {

	now := time.Now()

	if now.Sub(tl.IPLastTime) > tl.IPCountInterval {
		tl.IPbytes = 0
		tl.IPLastTime = time.Now()
	}

	tl.IPbytes += bytes
	tl.IPbytesTotal += bytes
	return tl.IPbytes, tl.IPLastTime
}

func (tl *TrafficCounter) StreamTotalBytes() uint64 {

	return tl.IPbytesTotal
}

func (tl *TrafficCounter) StreamCountInterval() time.Duration {

	return tl.IPCountInterval
}
