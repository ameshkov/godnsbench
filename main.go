package main

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/AdguardTeam/dnsproxy/upstream"
	"github.com/AdguardTeam/golibs/log"
	goFlags "github.com/jessevdk/go-flags"
	"github.com/miekg/dns"
)

// VersionString is the version that we'll print to the output. See the makefile
// for more details.
var VersionString = "undefined"

// printEveryNRecords regulates when we should print the intermediate results.
const printEveryNRecords = 100

// Options represents console arguments.
type Options struct {
	// Address of the server you want to bench.
	Address string `short:"a" long:"address" description:"Address of the DNS server you're trying to test. Note, that it should include the protocol (tls://, https://, quic://)" optional:"false"`

	// Connections is the number of connections you would like to open simultaneously.
	Connections int `short:"p" long:"parallel" description:"The number of connections you would like to open simultaneously" default:"1"`

	// Query is the host name you would like to resolve during the bench.
	Query string `short:"q" long:"query" description:"The host name you would like to resolve" default:"example.org"`

	// Timeout is timeout for a query.
	Timeout int `short:"t" long:"timeout" description:"Query timeout in seconds" default:"10"`

	// QueriesCount is the overall number of queries we should send.
	QueriesCount int `short:"c" long:"count" description:"The overall number of queries we should send" default:"10000"`

	// Log settings
	// --

	// Verbose defines whether we should write the DEBUG-level log or not.
	Verbose bool `short:"v" long:"verbose" description:"Verbose output (optional)" optional:"yes" optional-value:"true"`

	// LogOutput is the optional path to the log file.
	LogOutput string `short:"o" long:"output" description:"Path to the log file. If not set, write to stdout."`
}

func main() {
	for _, arg := range os.Args {
		if arg == "--version" || arg == "-v" {
			fmt.Printf("godnsbench version: %s\n", VersionString)
			os.Exit(0)
		}
	}

	options := &Options{}
	parser := goFlags.NewParser(options, goFlags.Default)
	_, err := parser.Parse()
	if err != nil {
		if flagsErr, ok := err.(*goFlags.Error); ok && flagsErr.Type == goFlags.ErrHelp {
			os.Exit(0)
		} else {
			os.Exit(1)
		}
	}

	run(options)
}

// runState represents
type runState struct {
	// startTime is the time when the test has been started.
	startTime time.Time
	// processed is the number of queries successfully processed.
	processed int
	// errors is the number of queries that failed.
	errors int
	// queriesToSend is the number of queries left to send.
	queriesToSend int

	// lastPrintedState is the last time we printed the intermediate state.
	// It is printed on every 100's query.
	lastPrintedState     time.Time
	lastPrintedProcessed int
	lastPrintedErrors    int

	// m protects all fields.
	m sync.Mutex
}

// qpsTotal returns the number of queries processed in one second.
func (r *runState) qpsTotal() (q float64) {
	r.m.Lock()
	defer r.m.Unlock()

	e := r.elapsed()
	return float64(r.processed+r.errors) / e.Seconds()
}

// elapsed returns total elapsed time.
func (r *runState) elapsed() (e time.Duration) {
	return time.Now().Sub(r.startTime)
}

// elapsedPerQuery returns elapsed time per query.
func (r *runState) elapsedPerQuery() (e time.Duration) {
	elapsed := r.elapsed()
	avgElapsed := elapsed
	if r.processed > 0 {
		avgElapsed = elapsed / time.Duration(r.processed)
	}
	return avgElapsed
}

// incProcessed increments processed number, returns the new value.
func (r *runState) incProcessed() (p int) {
	r.m.Lock()
	defer r.m.Unlock()
	r.processed++
	r.printIntermediateResults()
	return r.processed
}

// printIntermediateResults prints intermediate results if needed.  This method
// must be protected by the mutex on the outside.
func (r *runState) printIntermediateResults() {
	// Time to print the intermediate result and qps.
	queriesCount := r.processed + r.errors - r.lastPrintedProcessed - r.lastPrintedErrors

	if queriesCount%printEveryNRecords == 0 {
		startTime := r.lastPrintedState
		if r.lastPrintedState.IsZero() {
			startTime = r.startTime
		}

		elapsed := time.Now().Sub(startTime)
		qps := float64(queriesCount) / elapsed.Seconds()

		log.Info("Processed %d queries, errors: %d", r.processed, r.errors)
		log.Info("Queries per second: %f", qps)
		r.lastPrintedState = time.Now()
		r.lastPrintedProcessed = r.processed
		r.lastPrintedErrors = r.errors
	}
}

// incErrors increments errors number, returns the new value.
func (r *runState) incErrors() (e int) {
	r.m.Lock()
	defer r.m.Unlock()
	r.errors++
	r.printIntermediateResults()
	return r.errors
}

// decQueriesToSend decrements queriesToSend number, returns the new value.
func (r *runState) decQueriesToSend() (q int) {
	r.m.Lock()
	defer r.m.Unlock()
	r.queriesToSend--
	return r.queriesToSend
}

func run(options *Options) {
	if options.Verbose {
		log.SetLevel(log.DEBUG)
	}
	if options.LogOutput != "" {
		file, err := os.OpenFile(options.LogOutput, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
		if err != nil {
			log.Fatalf("cannot create a log file: %s", err)
		}
		defer file.Close() //nolint
		log.SetOutput(file)
	}

	log.Info("Run godnsbench with the following configuration")
	log.Info("Address: %s", options.Address)
	log.Info("Connections count: %d", options.Connections)
	log.Info("Query: %s", options.Query)
	log.Info("Queries to send: %d", options.QueriesCount)
	log.Info("Query timeout: %d seconds", options.Timeout)

	_, err := upstream.AddressToUpstream(options.Address, &upstream.Options{})
	if err != nil {
		log.Fatalf("The server address %s is invalid: %v", options.Address, err)
	}

	// Subscribe to the OS events.
	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, syscall.SIGINT, syscall.SIGTERM)

	state := &runState{
		startTime:     time.Now(),
		queriesToSend: options.QueriesCount + 1,
	}

	// Subscribe to the bench run close event.
	closeChannel := make(chan bool, 1)

	// Run it in a separate goroutine so that we could react to other signals.
	go func() {
		log.Info(
			"Starting the test and running %d connections in parallel",
			options.Connections,
		)
		var wg sync.WaitGroup
		for i := 0; i < options.Connections; i++ {
			wg.Add(1)
			go func() {
				runConnection(options, state)
				wg.Done()
			}()
		}
		wg.Wait()

		log.Info("Finished running all connections")
		close(closeChannel)
	}()

	select {
	case <-signalChannel:
		log.Info("The test has been interrupted.")
	case <-closeChannel:
		log.Info("The test has finished.")
	}

	log.Info("The test results are:")

	elapsed := state.elapsed()
	log.Info("Elapsed: %s", elapsed)
	log.Info("Average QPS: %f", state.qpsTotal())
	log.Info("Processed queries: %d", state.processed)
	log.Info("Average per query: %s", state.elapsedPerQuery())
	log.Info("Errors count: %d", state.errors)
}

func runConnection(options *Options, state *runState) {
	// Ignoring the error here since upstream address was already verified.
	u, _ := upstream.AddressToUpstream(options.Address, &upstream.Options{
		Timeout: time.Duration(options.Timeout) * time.Second,
	})

	queriesToSend := state.decQueriesToSend()
	for queriesToSend > 0 {
		m := &dns.Msg{
			MsgHdr: dns.MsgHdr{
				Id:               dns.Id(),
				RecursionDesired: true,
			},
			Question: []dns.Question{{
				Name:   dns.Fqdn(options.Query),
				Qtype:  dns.TypeA,
				Qclass: dns.ClassINET,
			}},
		}
		_, err := u.Exchange(m)

		if err == nil {
			_ = state.incProcessed()
		} else {
			_ = state.incErrors()
			log.Debug("error occurred: %v", err)

			// We should re-create the upstream in this case.
			u, _ = upstream.AddressToUpstream(options.Address, &upstream.Options{
				Timeout: time.Duration(options.Timeout) * time.Second,
			})
		}

		queriesToSend = state.decQueriesToSend()
	}
}
