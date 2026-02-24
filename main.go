package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Hosts         []string      `yaml:"hosts"`
	PingInterval  time.Duration `yaml:"ping_interval"`
	SpeedInterval time.Duration `yaml:"speed_interval"`
	Prometheus    struct {
		Port int `yaml:"port"`
	} `yaml:"prometheus"`
	Logging struct {
		Level string `yaml:"level"`
	} `yaml:"logging"`
}

type Metrics struct {
	pingLatency   *prometheus.GaugeVec
	packetLoss    *prometheus.GaugeVec
	downloadSpeed prometheus.Gauge
	uploadSpeed   prometheus.Gauge
	lastPingTime  *prometheus.GaugeVec
	lastSpeedTime prometheus.Gauge
}

var (
	config  Config
	metrics Metrics
	logger  *logrus.Logger
)

func init() {
	logger = logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
}

func loadConfig(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	return decoder.Decode(&config)
}

func initMetrics() {
	metrics.pingLatency = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "isp_ping_latency_ms",
			Help: "Ping latency in milliseconds",
		},
		[]string{"host"},
	)

	metrics.packetLoss = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "isp_packet_loss_percent",
			Help: "Packet loss percentage",
		},
		[]string{"host"},
	)

	metrics.downloadSpeed = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "isp_download_speed_mbps",
			Help: "Download speed in Mbps",
		},
	)

	metrics.uploadSpeed = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "isp_upload_speed_mbps",
			Help: "Upload speed in Mbps",
		},
	)

	metrics.lastPingTime = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "isp_last_ping_time",
			Help: "Timestamp of last ping measurement",
		},
		[]string{"host"},
	)

	metrics.lastSpeedTime = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "isp_last_speed_test_time",
			Help: "Timestamp of last speed test",
		},
	)

	prometheus.MustRegister(metrics.pingLatency)
	prometheus.MustRegister(metrics.packetLoss)
	prometheus.MustRegister(metrics.downloadSpeed)
	prometheus.MustRegister(metrics.uploadSpeed)
	prometheus.MustRegister(metrics.lastPingTime)
	prometheus.MustRegister(metrics.lastSpeedTime)
}

func startPingMonitor(ctx context.Context) {
	ticker := time.NewTicker(config.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for _, host := range config.Hosts {
				go func(h string) {
					latency, loss := pingHost(h)
					metrics.pingLatency.WithLabelValues(h).Set(latency)
					metrics.packetLoss.WithLabelValues(h).Set(loss)
					metrics.lastPingTime.WithLabelValues(h).Set(float64(time.Now().Unix()))
					logger.WithFields(logrus.Fields{
						"host":    h,
						"latency": latency,
						"loss":    loss,
					}).Info("Ping measurement completed")
				}(host)
			}
		}
	}
}

func startSpeedMonitor(ctx context.Context) {
	ticker := time.NewTicker(config.SpeedInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			download, upload := measureSpeed()
			metrics.downloadSpeed.Set(download)
			metrics.uploadSpeed.Set(upload)
			metrics.lastSpeedTime.Set(float64(time.Now().Unix()))
			logger.WithFields(logrus.Fields{
				"download_mbps": download,
				"upload_mbps":   upload,
			}).Info("Speed test completed")
		}
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: isp-checker <config.yaml>")
		os.Exit(1)
	}

	configFile := os.Args[1]
	if err := loadConfig(configFile); err != nil {
		logger.Fatalf("Failed to load config: %v", err)
	}

	level, err := logrus.ParseLevel(config.Logging.Level)
	if err != nil {
		logger.SetLevel(logrus.InfoLevel)
	} else {
		logger.SetLevel(level)
	}

	initMetrics()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start monitoring goroutines
	go startPingMonitor(ctx)
	go startSpeedMonitor(ctx)

	// Start Prometheus metrics server
	http.Handle("/metrics", promhttp.Handler())
	server := &http.Server{
		Addr: fmt.Sprintf(":%d", config.Prometheus.Port),
	}

	go func() {
		logger.Infof("Starting metrics server on port %d", config.Prometheus.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("Failed to start metrics server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// Give outstanding requests 5 seconds to complete
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Errorf("Server forced to shutdown: %v", err)
	}

	logger.Info("Server exited")
}
