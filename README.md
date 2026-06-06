# isp-checker

Checks ISP latencies and performs speed tests. Publishes results to a Prometheus-compatible endpoint.

## Metrics

The application exposes metrics at `/metrics` on the configured Prometheus port.

In addition to ISP measurements (`isp_ping_latency_ms`, `isp_packet_loss_percent`, `isp_download_speed_mbps`, etc.), it now exports self-observability metrics that can help diagnose memory growth on small devices like a Raspberry Pi:

- `isp_app_memory_alloc_bytes` - bytes allocated and still in use
- `isp_app_memory_heap_alloc_bytes` - heap bytes allocated and still in use
- `isp_app_memory_heap_inuse_bytes` - heap bytes marked in use
- `isp_app_memory_heap_idle_bytes` - heap bytes reserved but idle
- `isp_app_memory_heap_released_bytes` - heap bytes returned to the OS
- `isp_app_memory_stack_inuse_bytes` - stack bytes in use
- `isp_app_memory_sys_bytes` - bytes obtained from the OS by the Go runtime
- `isp_app_gc_count_total` - completed garbage collections
- `isp_app_goroutines` - current goroutine count
- `isp_app_uptime_seconds` - process uptime
- `isp_app_info` - app metadata

The default Prometheus Go/process collectors are also exposed by the existing Prometheus handler, including useful metrics such as `go_memstats_*`, `go_goroutines`, and `process_resident_memory_bytes`.
