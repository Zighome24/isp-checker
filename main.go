package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sync"
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

	appInfo prometheus.Gauge
	uptime  prometheus.GaugeFunc

	memoryAllocBytes        prometheus.GaugeFunc
	memoryHeapAllocBytes    prometheus.GaugeFunc
	memoryHeapInUseBytes    prometheus.GaugeFunc
	memoryHeapIdleBytes     prometheus.GaugeFunc
	memoryHeapReleasedBytes prometheus.GaugeFunc
	memoryStackInUseBytes   prometheus.GaugeFunc
	memorySysBytes          prometheus.GaugeFunc
	gcCount                 prometheus.GaugeFunc
	goroutines              prometheus.GaugeFunc
}

var (
	config    Config
	metrics   Metrics
	logger    *logrus.Logger
	startTime = time.Now()
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

	metrics.appInfo = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name:        "isp_app_info",
			Help:        "Static information about the isp-checker application",
			ConstLabels: prometheus.Labels{"version": "unknown", "go_version": runtime.Version()},
		},
	)
	metrics.appInfo.Set(1)

	metrics.uptime = prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "isp_app_uptime_seconds",
			Help: "Number of seconds isp-checker has been running",
		},
		func() float64 { return time.Since(startTime).Seconds() },
	)

	metrics.memoryAllocBytes = newMemStatsGauge("isp_app_memory_alloc_bytes", "Bytes allocated and still in use by isp-checker", func(m *runtime.MemStats) uint64 { return m.Alloc })
	metrics.memoryHeapAllocBytes = newMemStatsGauge("isp_app_memory_heap_alloc_bytes", "Heap bytes allocated and still in use by isp-checker", func(m *runtime.MemStats) uint64 { return m.HeapAlloc })
	metrics.memoryHeapInUseBytes = newMemStatsGauge("isp_app_memory_heap_inuse_bytes", "Heap bytes currently marked as in use by isp-checker", func(m *runtime.MemStats) uint64 { return m.HeapInuse })
	metrics.memoryHeapIdleBytes = newMemStatsGauge("isp_app_memory_heap_idle_bytes", "Heap bytes reserved but not currently in use by isp-checker", func(m *runtime.MemStats) uint64 { return m.HeapIdle })
	metrics.memoryHeapReleasedBytes = newMemStatsGauge("isp_app_memory_heap_released_bytes", "Heap bytes released back to the operating system by isp-checker", func(m *runtime.MemStats) uint64 { return m.HeapReleased })
	metrics.memoryStackInUseBytes = newMemStatsGauge("isp_app_memory_stack_inuse_bytes", "Stack bytes currently in use by isp-checker", func(m *runtime.MemStats) uint64 { return m.StackInuse })
	metrics.memorySysBytes = newMemStatsGauge("isp_app_memory_sys_bytes", "Bytes obtained from the operating system by the Go runtime", func(m *runtime.MemStats) uint64 { return m.Sys })
	metrics.gcCount = newMemStatsGauge("isp_app_gc_count_total", "Number of completed garbage collections", func(m *runtime.MemStats) uint64 { return uint64(m.NumGC) })
	metrics.goroutines = prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "isp_app_goroutines",
			Help: "Number of goroutines currently running in isp-checker",
		},
		func() float64 { return float64(runtime.NumGoroutine()) },
	)

	prometheus.MustRegister(metrics.pingLatency)
	prometheus.MustRegister(metrics.packetLoss)
	prometheus.MustRegister(metrics.downloadSpeed)
	prometheus.MustRegister(metrics.uploadSpeed)
	prometheus.MustRegister(metrics.lastPingTime)
	prometheus.MustRegister(metrics.lastSpeedTime)
	prometheus.MustRegister(metrics.appInfo)
	prometheus.MustRegister(metrics.uptime)
	prometheus.MustRegister(metrics.memoryAllocBytes)
	prometheus.MustRegister(metrics.memoryHeapAllocBytes)
	prometheus.MustRegister(metrics.memoryHeapInUseBytes)
	prometheus.MustRegister(metrics.memoryHeapIdleBytes)
	prometheus.MustRegister(metrics.memoryHeapReleasedBytes)
	prometheus.MustRegister(metrics.memoryStackInUseBytes)
	prometheus.MustRegister(metrics.memorySysBytes)
	prometheus.MustRegister(metrics.gcCount)
	prometheus.MustRegister(metrics.goroutines)
}

func newMemStatsGauge(name, help string, value func(*runtime.MemStats) uint64) prometheus.GaugeFunc {
	return prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: name,
			Help: help,
		},
		func() float64 {
			var stats runtime.MemStats
			runtime.ReadMemStats(&stats)
			return float64(value(&stats))
		},
	)
}

func startPingMonitor(ctx context.Context) {
	ticker := time.NewTicker(config.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			var wg sync.WaitGroup
			for _, host := range config.Hosts {
				wg.Add(1)
				go func(h string) {
					defer wg.Done()
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
			wg.Wait()
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
