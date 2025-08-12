package metrics

import (
	"sync/atomic"
	"time"
)

type Options struct {
	Tick time.Duration
}

type Snap struct {
	Elapsed time.Duration
	Bytes   int64
	Mbps    float64
}

type Meter struct {
	opts Options
	C    chan Snap
	stop chan struct{}
}

func New(opts Options) *Meter {
	if opts.Tick <= 0 {
		opts.Tick = time.Second
	}
	return &Meter{
		opts: opts,
		C:    make(chan Snap, 8),
		stop: make(chan struct{}),
	}
}

func (m *Meter) Start(total *atomic.Int64) {
	start := time.Now()
	var last int64
	t := time.NewTicker(m.opts.Tick)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			now := time.Now()
			cur := total.Load()
			delta := cur - last
			last = cur
			mbps := (float64(delta) / m.opts.Tick.Seconds()) / (1024 * 1024)
			s := Snap{
				Elapsed: now.Sub(start),
				Bytes:   cur,
				Mbps:    mbps,
			}
			select {
			case m.C <- s:
			default:
			}
		case <-m.stop:
			close(m.C)
			return
		}
	}
}

func (m *Meter) Stop() { close(m.stop) }
