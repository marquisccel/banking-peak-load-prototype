package metrics

import (
	"context"
	"math"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const fallbackMemoryLimitBytes = 512 * 1024 * 1024

// StartRuntimeCollector updates lightweight process resource metrics for the
// local prototype dashboard. It avoids requiring cAdvisor for CPU and memory.
func StartRuntimeCollector(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 5 * time.Second
	}

	memoryLimit := detectMemoryLimitBytes()
	AppMemoryLimitBytes.Set(float64(memoryLimit))

	var lastCPUSeconds float64
	var lastWall time.Time

	collect := func() {
		now := time.Now()
		cpuSeconds, ok := processCPUSeconds()
		if ok && !lastWall.IsZero() {
			wallSeconds := now.Sub(lastWall).Seconds()
			if wallSeconds > 0 {
				cpuRatio := (cpuSeconds - lastCPUSeconds) / wallSeconds / float64(runtime.GOMAXPROCS(0))
				AppCPUUtilizationRatio.Set(math.Max(0, cpuRatio))
			}
		}
		if ok {
			lastCPUSeconds = cpuSeconds
			lastWall = now
		}

		var mem runtime.MemStats
		runtime.ReadMemStats(&mem)
		usage := float64(mem.Sys)
		AppMemoryUsageBytes.Set(usage)
		AppMemoryUtilizationRatio.Set(usage / float64(memoryLimit))
	}

	collect()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			collect()
		}
	}
}

func processCPUSeconds() (float64, bool) {
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil {
		return 0, false
	}
	user := float64(usage.Utime.Sec) + float64(usage.Utime.Usec)/1e6
	system := float64(usage.Stime.Sec) + float64(usage.Stime.Usec)/1e6
	return user + system, true
}

func detectMemoryLimitBytes() uint64 {
	if limit := parsePositiveUint(os.Getenv("APP_MEMORY_LIMIT_BYTES")); limit > 0 {
		return limit
	}

	for _, path := range []string{
		"/sys/fs/cgroup/memory.max",
		"/sys/fs/cgroup/memory/memory.limit_in_bytes",
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		raw := strings.TrimSpace(string(data))
		if raw == "max" {
			continue
		}
		if limit := parsePositiveUint(raw); limit > 0 && limit < 1<<60 {
			return limit
		}
	}

	return fallbackMemoryLimitBytes
}

func parsePositiveUint(raw string) uint64 {
	if raw == "" {
		return 0
	}
	value, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0
	}
	return value
}
