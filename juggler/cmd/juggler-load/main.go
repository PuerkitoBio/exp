// Command juggler-load is a juggler load generator. It runs a
// number of client connections to a juggler server, and for a
// given duration, makes calls and collects results.
package main

import (
	"encoding/json"
	"flag"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"text/template"
	"time"

	"golang.org/x/net/context"

	"github.com/PuerkitoBio/exp/juggler"
	"github.com/PuerkitoBio/exp/juggler/client"
	"github.com/PuerkitoBio/exp/juggler/msg"
	"github.com/gorilla/websocket"
)

var (
	addrFlag        = flag.String("addr", "ws://localhost:9000/ws", "Server `address`.")
	subprotoFlag    = flag.String("proto", "juggler.0", "Websocket `subprotocol`.")
	connFlag        = flag.Int("c", 100, "Number of `connections`.")
	durationFlag    = flag.Duration("d", 10*time.Second, "Run `duration`.")
	callRateFlag    = flag.Duration("r", 100*time.Millisecond, "Call `rate` per connection.")
	callTimeoutFlag = flag.Duration("t", time.Second, "Call `timeout`.")
	uriFlag         = flag.String("u", "test.delay", "Call `URI`.")
	payloadFlag     = flag.String("p", "100", "Call `payload`.")
	helpFlag        = flag.Bool("help", false, "Show help.")
)

var (
	fnMap = template.FuncMap{
		"sub": subFn,
	}

	tpl = template.Must(template.New("output").Funcs(fnMap).Parse(`
--- CONFIGURATION

Address:    {{ .Run.Addr }}
Protocol:   {{ .Run.Protocol }}
URI:        {{ .Run.URI }}
Call Delay: {{ .Run.Payload }}

Connections: {{ .Run.Conns }}
Rate:        {{ .Run.Rate | printf "%s" }}
Timeout:     {{ .Run.Timeout | printf "%s" }}
Duration:    {{ .Run.Duration | printf "%s" }}

--- STATISTICS

Actual Duration: {{ .Run.ActualDuration | printf "%s" }}
Calls:           {{ .Run.Calls }}
OK:              {{ .Run.OK }}
Errors:          {{ .Run.Err }}
Results:         {{ .Run.Res }}
Expired:         {{ .Run.Exp }}

--- SERVER STATISTICS

                Before          After          Diff.
Alloc:         {{ .Before.Memstats.Alloc | printf "% -15d" }} {{ .After.Memstats.Alloc | printf "% -15d" }} {{ sub .After.Memstats.Alloc .Before.Memstats.Alloc }}
TotalAlloc:    {{ .Before.Memstats.TotalAlloc | printf "% -15d" }} {{ .After.Memstats.TotalAlloc | printf "% -15d" }} {{ sub .After.Memstats.TotalAlloc .Before.Memstats.TotalAlloc }}
Mallocs:       {{ .Before.Memstats.Mallocs | printf "% -15d" }} {{ .After.Memstats.Mallocs | printf "% -15d" }} {{ sub .After.Memstats.Mallocs .Before.Memstats.Mallocs }} 
Frees:         {{ .Before.Memstats.Frees | printf "% -15d" }} {{ .After.Memstats.Frees | printf "% -15d" }} {{ sub .After.Memstats.Frees .Before.Memstats.Frees }}
HeapAlloc:     {{ .Before.Memstats.HeapAlloc | printf "% -15d" }} {{ .After.Memstats.HeapAlloc | printf "% -15d" }} {{ sub .After.Memstats.HeapAlloc .Before.Memstats.HeapAlloc }}
HeapInuse:     {{ .Before.Memstats.HeapInuse | printf "% -15d" }} {{ .After.Memstats.HeapInuse | printf "% -15d" }} {{ sub .After.Memstats.HeapInuse .Before.Memstats.HeapInuse }}
HeapObjects:   {{ .Before.Memstats.HeapObjects | printf "% -15d" }} {{ .After.Memstats.HeapObjects | printf "% -15d" }} {{ sub .After.Memstats.HeapObjects .Before.Memstats.HeapObjects }}
StackInuse:    {{ .Before.Memstats.StackInuse | printf "% -15d" }} {{ .After.Memstats.StackInuse | printf "% -15d" }} {{ sub .After.Memstats.StackInuse .Before.Memstats.StackInuse }}
NumGC:         {{ .Before.Memstats.NumGC | printf "% -15d" }} {{ .After.Memstats.NumGC | printf "% -15d" }} {{ sub .After.Memstats.NumGC .Before.Memstats.NumGC }}
PauseTotalNs:  {{ .Before.Memstats.PauseTotalNs | printf "% -15d" }} {{ .After.Memstats.PauseTotalNs | printf "% -15d" }} {{ sub .After.Memstats.PauseTotalNs .Before.Memstats.PauseTotalNs }}

`))
)

func subFn(a, b int) int {
	return a - b
}

type templateStats struct {
	Run    *runStats
	Before *expVars
	After  *expVars
}

type runStats struct {
	Addr     string
	Protocol string
	URI      string
	Payload  string

	Conns          int
	Rate           time.Duration
	Timeout        time.Duration
	Duration       time.Duration
	ActualDuration time.Duration

	Calls int64
	OK    int64
	Err   int64
	Res   int64
	Exp   int64
}

type expVars struct {
	Juggler struct {
		ActiveConnGoros int
		ActiveConns     int
		CallMsgs        int
		ErrMsgs         int
		Msgs            int
		OKMsgs          int
		ReadMsgs        int
		RecoveredPanics int
		ResMsgs         int
		TotalConnGoros  int
		TotalConns      int
		WriteMsgs       int
	}

	Memstats struct {
		Alloc        int
		TotalAlloc   int
		Mallocs      int
		Frees        int
		HeapAlloc    int
		HeapInuse    int
		HeapObjects  int
		StackInuse   int
		NumGC        int
		PauseTotalNs int
	}
}

func main() {
	flag.Parse()
	if *helpFlag {
		flag.Usage()
		return
	}

	log.SetFlags(0)

	if *connFlag <= 0 {
		log.Fatalf("invalid -c value, must be greater than 0")
	}

	stats := &runStats{
		Addr:     *addrFlag,
		Protocol: *subprotoFlag,
		URI:      *uriFlag,
		Payload:  *payloadFlag,
		Conns:    *connFlag,
		Rate:     *callRateFlag,
		Timeout:  *callTimeoutFlag,
		Duration: *durationFlag,
	}

	// TODO : collect expvars before and after if flag is set
	parsed, err := url.Parse(stats.Addr)
	if err != nil {
		log.Fatalf("failed to parse --addr: %v", err)
	}
	parsed.Scheme = "http"
	parsed.Path = "/debug/vars"
	before := getExpVars(parsed)

	clientStarted := make(chan struct{})
	wg := sync.WaitGroup{}
	wg.Add(stats.Conns)

	stop := make(chan struct{})
	for i := 0; i < stats.Conns; i++ {
		go runClient(stats, &wg, clientStarted, stop)
	}

	// start clients with some jitter, up to 10ms
	log.Printf("%d connections started...", stats.Conns)
	start := time.Now()
	for i := 0; i < stats.Conns; i++ {
		<-time.After(time.Duration(rand.Intn(int(10 * time.Millisecond))))
		<-clientStarted
	}

	// run for the requested duration and signal stop
	<-time.After(stats.Duration)
	close(stop)
	log.Printf("stopping...")

	// wait for completion
	done := make(chan struct{})
	go func() {
		select {
		case <-done:
			return
		case <-time.After(5 * time.Second):
			log.Fatalf("failed to stop clients")
		}
	}()
	wg.Wait()
	close(done)
	end := time.Now()
	stats.ActualDuration = end.Sub(start)
	log.Printf("stopped.")

	after := getExpVars(parsed)

	ts := templateStats{Run: stats, Before: before, After: after}
	if err := tpl.Execute(os.Stdout, ts); err != nil {
		log.Fatalf("template.Execute failed: %v", err)
	}
}

func getExpVars(u *url.URL) *expVars {
	res, err := http.Get(u.String())
	if err != nil {
		log.Fatalf("failed to fetch /debug/vars: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		log.Fatalf("failed to fetch /debug/vars: %d %s", res.StatusCode, res.Status)
	}

	var ev expVars
	if err := json.NewDecoder(res.Body).Decode(&ev); err != nil {
		log.Fatalf("failed to decode expvars: %v", err)
	}
	return &ev
}

func runClient(stats *runStats, wg *sync.WaitGroup, started chan<- struct{}, stop <-chan struct{}) {
	defer wg.Done()

	var wgResults sync.WaitGroup

	cli, err := client.Dial(
		&websocket.Dialer{Subprotocols: []string{stats.Protocol}},
		stats.Addr, nil,
		client.SetLogFunc(juggler.DiscardLog),
		client.SetHandler(client.HandlerFunc(func(ctx context.Context, c *client.Client, m msg.Msg) {
			switch m.Type() {
			case msg.ResMsg:
				atomic.AddInt64(&stats.Res, 1)
			case client.ExpMsg:
				atomic.AddInt64(&stats.Exp, 1)
			case msg.OKMsg:
				atomic.AddInt64(&stats.OK, 1)
				return
			case msg.ErrMsg:
				atomic.AddInt64(&stats.Err, 1)
			default:
				log.Fatalf("unexpected message type %s", m.Type())
			}
			wgResults.Done()
		})))

	if err != nil {
		log.Fatalf("Dial failed: %v", err)
	}

	var after time.Duration
	started <- struct{}{}
loop:
	for {
		select {
		case <-stop:
			break loop
		case <-time.After(after):
		}

		wgResults.Add(1)
		atomic.AddInt64(&stats.Calls, 1)
		_, err := cli.Call(stats.URI, stats.Payload, stats.Timeout)
		if err != nil {
			log.Fatalf("Call failed: %v", err)
		}
		after = stats.Rate
	}
	// wait for sent calls to return or expire
	wgResults.Wait()

	if err := cli.Close(); err != nil {
		log.Fatalf("Close failed: %v", err)
	}
}