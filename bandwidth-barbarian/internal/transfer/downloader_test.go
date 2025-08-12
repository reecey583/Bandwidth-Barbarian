package transfer

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestMetricsMath(t *testing.T) {
	var total atomic.Int64
	total.Store(0)
	total.Add(1024 * 1024 * 100) // 100 MiB
	// pretend 2 seconds elapsed, expected MBps about 50
	elapsed := 2.0
	mbps := (float64(total.Load()) / elapsed) / (1024 * 1024)
	if mbps < 49.0 || mbps > 51.0 {
		t.Fatalf("mbps math off: %.2f", mbps)
	}
	time.Sleep(10 * time.Millisecond)
}
