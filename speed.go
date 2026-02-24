package main

import (
	"time"

	"github.com/showwin/speedtest-go/speedtest"
	"github.com/sirupsen/logrus"
)

func measureSpeed() (float64, float64) {
	serverList, err := speedtest.FetchServers()
	if err != nil {
		logger.WithError(err).Error("Failed to fetch server list for speed test")
		return -1, -1
	}

	targets, err := serverList.FindServer([]int{})
	if err != nil {
		logger.WithError(err).Error("Failed to find speed test server")
		return -1, -1
	}

	if len(targets) == 0 {
		logger.Error("No speed test servers found")
		return -1, -1
	}

	server := targets[0]

	err = server.PingTest(func(latency time.Duration) {})
	if err != nil {
		logger.WithError(err).Error("Failed to ping speed test server")
		return -1, -1
	}

	err = server.DownloadTest()
	if err != nil {
		logger.WithError(err).Error("Failed to run download speed test")
		return -1, -1
	}

	err = server.UploadTest()
	if err != nil {
		logger.WithError(err).Error("Failed to run upload speed test")
		return -1, -1
	}

	downloadSpeed := float64(server.DLSpeed)
	uploadSpeed := float64(server.ULSpeed)

	logger.WithFields(logrus.Fields{
		"server":        server.Host,
		"download_mbps": downloadSpeed,
		"upload_mbps":   uploadSpeed,
		"ping_ms":       server.Latency.Milliseconds(),
	}).Info("Speed test completed")

	return downloadSpeed, uploadSpeed
}
