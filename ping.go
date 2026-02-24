package main

import (
	"time"

	"github.com/go-ping/ping"
	"github.com/sirupsen/logrus"
)

func pingHost(host string) (float64, float64) {
	pinger, err := ping.NewPinger(host)
	if err != nil {
		logger.WithError(err).Errorf("Failed to create pinger for %s", host)
		return -1, 100
	}

	pinger.Count = 4
	pinger.Timeout = 5 * time.Second
	pinger.Interval = 1 * time.Second

	err = pinger.Run()
	if err != nil {
		logger.WithError(err).Errorf("Failed to ping %s", host)
		return -1, 100
	}

	stats := pinger.Statistics()

	latency := float64(stats.AvgRtt.Nanoseconds()) / 1000000 // Convert to milliseconds
	packetLoss := stats.PacketLoss

	logger.WithFields(logrus.Fields{
		"host":       host,
		"packets_tx": stats.PacketsSent,
		"packets_rx": stats.PacketsRecv,
		"avg_rtt":    stats.AvgRtt,
		"min_rtt":    stats.MinRtt,
		"max_rtt":    stats.MaxRtt,
	}).Debug("Ping statistics")

	return latency, packetLoss
}
