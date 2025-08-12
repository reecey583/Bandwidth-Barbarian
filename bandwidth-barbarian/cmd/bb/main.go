package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"bandwidth-barbarian/internal/metrics"
	"bandwidth-barbarian/internal/sink"
	"bandwidth-barbarian/internal/transfer"
)

type Report struct {
	Mode        string   `json:"mode"`
	URLs        []string `json:"urls"`
	Conns       int      `json:"conns"`
	DurationSec int64    `json:"duration_sec"`
	Bytes       int64    `json:"bytes"`
	MBps        float64  `json:"mbps"`
	GBytes      float64  `json:"gbytes"`
	StartedAt   string   `json:"started_at"`
	FinishedAt  string   `json:"finished_at"`
}

func writeReport(r Report) {
	data, _ := json.MarshalIndent(r, "", "  ")
	_ = os.WriteFile("bb-report.json", data, 0o644)
}

func main() {
	log.SetFlags(0)

	if len(os.Args) < 2 {
		usage()
		return
	}

	switch os.Args[1] {
	case "dl":
		runDL(os.Args[2:])
	case "ul":
		runUL(os.Args[2:])
	case "sink":
		runSink(os.Args[2:])
	default:
		usage()
	}
}

func usage() {
	fmt.Println(`bb - bandwidth barbarian

Usage:
  bb dl   --url URL [--url URL ...] [--conns 16] [--time 10m] [--loop] [--i-understand]
  bb ul   --url URL [--conns 16] [--time 10m]
  bb sink --port 8080

Examples:
  bb dl --url https://speed.hetzner.de/10GB.bin --conns 64 --time 5m --loop --i-understand
  bb sink --port 8080
  bb ul --url http://127.0.0.1:8080/upload --conns 32 --time 10m`)
}

func runDL(args []string) {
	fs := flag.NewFlagSet("dl", flag.ExitOnError)
	var urls multi
	var conns int
	var durStr string
	var loop bool
	var understand bool
	fs.Var(&urls, "url", "download url, repeatable")
	fs.IntVar(&conns, "conns", 16, "concurrent connections total")
	fs.StringVar(&durStr, "time", "5m", "duration like 30s 5m 1h")
	fs.BoolVar(&loop, "loop", false, "loop downloads")
	fs.BoolVar(&understand, "i-understand", false, "confirm you have permission to hit these urls hard")
	_ = fs.Parse(args)

	if len(urls) == 0 {
		// default safe demo url
		urls = append(urls, "https://speed.hetzner.de/10GB.bin")
	}
	if !understand {
		fmt.Println("add --i-understand to confirm you have permission to test against the provided urls")
		os.Exit(2)
	}
	if conns < 1 {
		conns = 1
	}
	dur, err := time.ParseDuration(durStr)
	if err != nil {
		log.Fatalf("bad --time: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), dur)
	defer cancel()

	// ctrl-c support
	ctx = withSignals(ctx, cancel)

	var total atomic.Int64
	mtx := metrics.New(metrics.Options{Tick: time.Second})
	go func() {
		for m := range mtx.C {
			// live print
			fmt.Printf("\rtime %s  mbps %.2f  total %.2f GB   ",
				m.Elapsed.Round(time.Second).String(), m.Mbps, float64(m.Bytes)/1e9)
		}
		fmt.Println()
	}()

	start := time.Now()
	go mtx.Start(&total)

	err = transfer.Download(ctx, urls, conns, loop, &total)
	end := time.Now()
	mtx.Stop()

	if err != nil && ctx.Err() == nil {
		log.Printf("download ended with error: %v", err)
	}

	r := Report{
		Mode:        "download",
		URLs:        urls,
		Conns:       conns,
		DurationSec: int64(end.Sub(start).Seconds()),
		Bytes:       total.Load(),
		MBps:        (float64(total.Load()) / end.Sub(start).Seconds()) / (1024 * 1024),
		GBytes:      float64(total.Load()) / 1e9,
		StartedAt:   start.Format(time.RFC3339),
		FinishedAt:  end.Format(time.RFC3339),
	}
	writeReport(r)
	fmt.Printf("\nDone. avg MBps %.2f  total %.2f GB\n", r.MBps, r.GBytes)
}

func runUL(args []string) {
	fs := flag.NewFlagSet("ul", flag.ExitOnError)
	var url string
	var conns int
	var durStr string
	fs.StringVar(&url, "url", "http://127.0.0.1:8080/upload", "upload target url")
	fs.IntVar(&conns, "conns", 16, "concurrent connections total")
	fs.StringVar(&durStr, "time", "5m", "duration like 30s 5m 1h")
	_ = fs.Parse(args)

	dur, err := time.ParseDuration(durStr)
	if err != nil {
		log.Fatalf("bad --time: %v", err)
	}
	if conns < 1 {
		conns = 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), dur)
	defer cancel()
	ctx = withSignals(ctx, cancel)

	var total atomic.Int64
	mtx := metrics.New(metrics.Options{Tick: time.Second})
	go func() {
		for m := range mtx.C {
			fmt.Printf("\rtime %s  mbps %.2f  total %.2f GB   ",
				m.Elapsed.Round(time.Second).String(), m.Mbps, float64(m.Bytes)/1e9)
		}
		fmt.Println()
	}()

	start := time.Now()
	go mtx.Start(&total)

	err = transfer.Upload(ctx, url, conns, &total)
	end := time.Now()
	mtx.Stop()

	if err != nil && ctx.Err() == nil {
		log.Printf("upload ended with error: %v", err)
	}

	r := Report{
		Mode:        "upload",
		URLs:        []string{url},
		Conns:       conns,
		DurationSec: int64(end.Sub(start).Seconds()),
		Bytes:       total.Load(),
		MBps:        (float64(total.Load()) / end.Sub(start).Seconds()) / (1024 * 1024),
		GBytes:      float64(total.Load()) / 1e9,
		StartedAt:   start.Format(time.RFC3339),
		FinishedAt:  end.Format(time.RFC3339),
	}
	writeReport(r)
	fmt.Printf("\nDone. avg MBps %.2f  total %.2f GB\n", r.MBps, r.GBytes)
}

func runSink(args []string) {
	fs := flag.NewFlagSet("sink", flag.ExitOnError)
	var port int
	fs.IntVar(&port, "port", 8080, "listen port")
	_ = fs.Parse(args)

	addr := fmt.Sprintf(":%d", port)
	fmt.Printf("sink listening on %s\n", addr)
	if err := sink.Run(addr); err != nil {
		log.Fatal(err)
	}
}

type multi []string

func (m *multi) String() string { return strings.Join(*m, ",") }
func (m *multi) Set(v string) error {
	*m = append(*m, v)
	return nil
}

func withSignals(ctx context.Context, cancel context.CancelFunc) context.Context {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-c
		cancel()
	}()
	return ctx
}
