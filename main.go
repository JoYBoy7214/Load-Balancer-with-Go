package main

import (
	ServerInfos "LoadBalancer/server"
	"context"
	"fmt"
	"log"
	"math"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"sync/atomic"
	"time"
)

const (
	Attempts int = iota
	Retry
)

type Backend struct {
	URL          *url.URL
	Alive        bool
	mux          sync.RWMutex
	ReverseProxy *httputil.ReverseProxy
	connections  int64
	weights      int64
}

func (b *Backend) IsAlive() (alive bool) {
	b.mux.RLock()
	alive = b.Alive
	b.mux.RUnlock()
	return alive
}

func (b *Backend) SetAlive(alive bool) {
	b.mux.Lock()
	b.Alive = alive
	b.mux.Unlock()

}

type ServerPool struct {
	Backends []*Backend
	current  uint64
}

func (s *ServerPool) NextIndex() int {
	return int(atomic.AddUint64(&s.current, uint64(1)) % uint64(len(s.Backends)))
}

func (s *ServerPool) GetNextPeer() *Backend {
	next := s.NextIndex()
	l := next + len(s.Backends)
	for i := next; i < l; i++ {
		ind := i % len(s.Backends)
		if s.Backends[ind].IsAlive() {
			atomic.StoreUint64(&s.current, uint64(ind))
			return s.Backends[ind]
		}
	}
	return nil
}

func (s *ServerPool) GetNextPeerForLeastConnections() *Backend {
	min := math.MaxFloat64
	var peer *Backend = nil
	for _, b := range s.Backends {
		if b.IsAlive() {
			if min > float64(float64(atomic.LoadInt64(&b.connections))/float64(b.weights)) {
				min = float64(float64(atomic.LoadInt64(&b.connections)) / float64(b.weights))
				peer = b
			}
		}
	}
	return peer
}

func (s *ServerPool) AddBackend(b *Backend) {
	s.Backends = append(s.Backends, b)
}

func (s *ServerPool) MarkBackendStatus(backendUrl *url.URL, alive bool) {
	for _, b := range s.Backends {
		if b.URL.String() == backendUrl.String() {
			b.SetAlive(alive)
			break
		}
	}
}

func (s *ServerPool) HealthCheck() {
	for _, b := range s.Backends {
		status := "UP"
		alive := isBackendAlive(b.URL)
		b.SetAlive(alive)
		if !alive {
			status = "DOWN"
		}
		log.Printf("%s [%s]\n", b.URL, status)
	}
}

func GetRetryFromContext(r *http.Request) int {
	if retry, ok := r.Context().Value(Retry).(int); ok {
		return retry
	}
	return 0
}

func GetAttemptsFromContext(r *http.Request) int {
	if attempts, ok := r.Context().Value(Attempts).(int); ok {
		return attempts
	}
	return 1
}

func isBackendAlive(u *url.URL) bool {
	timeout := 2 * time.Second
	conn, err := net.DialTimeout("tcp", u.Host, timeout)
	if err != nil {
		log.Println("site unreachable ,error: ", err)
		return false
	}
	defer conn.Close()
	return true
}

func healthCheck() {
	t := time.NewTicker(2 * time.Minute)
	for {
		select {
		case <-t.C:
			log.Println("Starting health check...")
			serverPoolRR.HealthCheck()
			serverPoolLC.HealthCheck()
			log.Println("Health check completed")
		}
	}
}
func lb(w http.ResponseWriter, r *http.Request) {
	attempts := GetAttemptsFromContext(r)
	if attempts > 3 {
		log.Printf("%s(%s) Max attempts reached, terminating\n", r.RemoteAddr, r.URL.Path)
		http.Error(w, "Service not available", http.StatusServiceUnavailable)
		return
	}
	peer := serverPoolRR.GetNextPeer()
	if peer != nil {
		peer.ReverseProxy.ServeHTTP(w, r)
		//this act as http.Handler It rewrites the request using its Director function.
		///It sends the request to the backend using its Transport.
		//It copies the backend’s response back to the original client (ResponseWriter).
		return
	}
	http.Error(w, "service not available", http.StatusServiceUnavailable)
}
func lbWithLeastConnections(w http.ResponseWriter, r *http.Request) {
	attempts := GetAttemptsFromContext(r)
	if attempts > 3 {
		log.Printf("%s(%s) Max attempts reached, terminating\n", r.RemoteAddr, r.URL.Path)
		http.Error(w, "Service not available", http.StatusServiceUnavailable)
		return
	}
	peer := serverPoolLC.GetNextPeerForLeastConnections()
	if peer != nil {
		atomic.AddInt64(&peer.connections, int64(1))
		defer atomic.AddInt64(&peer.connections, int64(-1))
		peer.ReverseProxy.ServeHTTP(w, r)

		return
	}
	http.Error(w, "service not available", http.StatusServiceUnavailable)
}
func startLeastConnection() {
	serverLeastConnection := http.Server{
		Addr:    ":3031",
		Handler: http.HandlerFunc(lbWithLeastConnections),
	}
	if err := serverLeastConnection.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
func configRR(serverInfo ServerInfos.ServerInfos) {

	for _, tok := range serverInfo.Backends {
		serverUrl, err := url.Parse(tok.Url)
		if err != nil {
			log.Fatal(err)
		}
		proxy := httputil.NewSingleHostReverseProxy(serverUrl)
		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, e error) {
			log.Printf("[%s] %s\n", serverUrl.Host, e.Error())
			retries := GetRetryFromContext(r)
			if retries < 3 {
				select {
				case <-time.After(10 * time.Millisecond):
					ctx := context.WithValue(r.Context(), Retry, retries+1)
					proxy.ServeHTTP(w, r.WithContext(ctx))
				}
				return
			}

			// after 3 retries, mark this backend as down
			serverPoolRR.MarkBackendStatus(serverUrl, false)

			attempts := GetAttemptsFromContext(r)
			log.Printf("%s(%s) Attempting retry %d\n", r.RemoteAddr, r.URL.Path, attempts)
			ctx := context.WithValue(r.Context(), Attempts, attempts+1)
			lb(w, r.WithContext(ctx))
		}
		serverPoolRR.AddBackend(&Backend{
			URL:          serverUrl,
			Alive:        true,
			ReverseProxy: proxy,
			connections:  0,
			weights:      int64(tok.Weight),
		})
		log.Printf("configured server: %s\n ", serverUrl)
	}
	server := http.Server{
		Addr:    fmt.Sprintf(":%d", serverInfo.Port),
		Handler: http.HandlerFunc(lb),
	}
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
	log.Printf("RR Load Balancer started at :%d\n", serverInfo.Port)
}
func configLC(serverInfo ServerInfos.ServerInfos) {
	for _, tok := range serverInfo.Backends {
		serverUrl, err := url.Parse(tok.Url)
		if err != nil {
			log.Fatal(err)
		}
		proxy := httputil.NewSingleHostReverseProxy(serverUrl)
		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, e error) {
			log.Printf("[%s] %s\n", serverUrl.Host, e.Error())
			retries := GetRetryFromContext(r)
			if retries < 3 {
				select {
				case <-time.After(10 * time.Millisecond):
					ctx := context.WithValue(r.Context(), Retry, retries+1)
					proxy.ServeHTTP(w, r.WithContext(ctx))
				}
				return
			}

			// after 3 retries, mark this backend as down
			serverPoolLC.MarkBackendStatus(serverUrl, false)

			attempts := GetAttemptsFromContext(r)
			log.Printf("%s(%s) Attempting retry %d\n", r.RemoteAddr, r.URL.Path, attempts)
			ctx := context.WithValue(r.Context(), Attempts, attempts+1)
			lbWithLeastConnections(w, r.WithContext(ctx))
		}
		serverPoolLC.AddBackend(&Backend{
			URL:          serverUrl,
			Alive:        true,
			ReverseProxy: proxy,
			connections:  0,
			weights:      int64(tok.Weight),
		})
		log.Printf("configured server: %s\n ", serverUrl)
	}
	server := http.Server{
		Addr:    fmt.Sprintf(":%d", serverInfo.Port),
		Handler: http.HandlerFunc(lbWithLeastConnections),
	}
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
	log.Printf("LC Load Balancer started at :%d\n", serverInfo.Port)
}

var serverPoolRR ServerPool
var serverPoolLC ServerPool

func main() {
	var sInfos []ServerInfos.ServerInfos
	sInfos = ServerInfos.GetServerInfo("serverConfig.json")
	var sInfosRR ServerInfos.ServerInfos
	var sInfoLC ServerInfos.ServerInfos
	for _, s := range sInfos {
		if s.Strategy == "round-robin" {
			sInfosRR = s
		} else {
			sInfoLC = s
		}
	}
	go configLC(sInfoLC)
	go healthCheck()
	configRR(sInfosRR)

}
