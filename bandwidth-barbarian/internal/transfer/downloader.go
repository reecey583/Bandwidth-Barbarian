package transfer

import (
	"context"
	"errors"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

func Download(ctx context.Context, urls []string, conns int, loop bool, total *atomic.Int64) error {
	if len(urls) == 0 {
		return errors.New("no urls")
	}
	cl := &http.Client{
		Timeout: 60 * time.Second,
	}

	var wg sync.WaitGroup
	errCh := make(chan error, conns)

	// simple round robin over urls
	for i := 0; i < conns; i++ {
		u := urls[i%len(urls)]
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			for {
				req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
				resp, err := cl.Do(req)
				if err != nil {
					select {
					case errCh <- err:
					default:
					}
					// small sleep to avoid hot loop on errors
					select {
					case <-time.After(500 * time.Millisecond):
					case <-ctx.Done():
						return
					}
					continue
				}
				// read body to /dev/null
				n, _ := io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
				total.Add(n)

				if ctx.Err() != nil {
					return
				}
				if !loop {
					// one pass per worker
					return
				}
			}
		}(u)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		return nil
	case <-done:
		return nil
	case err := <-errCh:
		return err
	}
}
