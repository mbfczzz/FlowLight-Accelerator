package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"log"
	"net"
	"net/http"
	"time"
)

func main() {
	tcp := flag.String("http", "127.0.0.1:9080", "HTTP probe listener; put behind TLS")
	udp := flag.String("udp", "0.0.0.0:9081", "UDP echo probe listener")
	flag.Parse()
	go func() {
		conn, e := net.ListenPacket("udp", *udp)
		if e != nil {
			log.Fatal(e)
		}
		defer conn.Close()
		b := make([]byte, 128)
		for {
			n, peer, e := conn.ReadFrom(b)
			if e != nil {
				return
			}
			if n == 28 && bytes.Equal(b[:4], []byte("FLA1")) {
				_, _ = conn.WriteTo(b[:n], peer)
			}
		}
	}()
	m := http.NewServeMux()
	m.HandleFunc("GET /probe", func(w http.ResponseWriter, r *http.Request) {
		ip, _, _ := net.SplitHostPort(r.RemoteAddr)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"ip": ip})
	})
	srv := &http.Server{Addr: *tcp, Handler: m, ReadHeaderTimeout: 5 * time.Second}
	log.Fatal(srv.ListenAndServe())
}
