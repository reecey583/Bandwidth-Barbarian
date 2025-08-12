package transfer

import (
	"context"
	"crypto/rand"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type randReader struct{}

func (r randReader) Read(p []byte) (int, error) {
	// fill with random bytes
	_, _ = rand.Read(p)
	return len(p), nil
}

func Upload(ctx context.Context, url string, conns int, total *atomic.Int64) error {
	cl := &http.Client{
		Timeout: 0, // stream until context cancel
	}
	var wg sync.WaitGroup

	body := io.NopCloser(io.LimitReader(randReader{}, 1<<62)) // huge stream

	for i := 0; i < conns; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				req, _ := http.NewRequestWithContext(ctx, "POST", url, body)
				req.Header.Set("Content-Type", "application/octet-stream")
				resp, err := cl.Do(req)
				if err != nil {
					select {
					case <-time.After(500 * time.Millisecond):
					case <-ctx.Done():
						return
					}
					continue
				}
				n, _ := io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
				total.Add(n)
				select {
				case <-ctx.Done():
					return
				default:
				}
				// some servers keep conn alive, but we just loop to ensure pressure
				if strings.HasPrefix(url, "http://127.0.0.1") || strings.HasPrefix(url, "http://localhost") {
					// instant retry
				} else {
					time.Sleep(200 * time.Millisecond)
				}
			}
		}()
	}
	wg.Wait()
	return nil
}
