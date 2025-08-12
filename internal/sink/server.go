package sink

import (
	"fmt"
	"io"
	"log"
	"net/http"
)

func Run(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// accept PUT or POST, discard body
		if r.Method != http.MethodPut && r.Method != http.MethodPost {
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, "ok")
			return
		}
		n, _ := io.Copy(io.Discard, r.Body)
		r.Body.Close()
		log.Printf("sink: received %d bytes from %s %s", n, r.Method, r.RemoteAddr)
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})
	return http.ListenAndServe(addr, mux)
}
