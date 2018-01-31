package ldapwatch

import (
	"log"
	"os"
	"os/signal"
	"time"

	ldap "gopkg.in/ldap.v2"
)

// Searcher ...
type Searcher interface {
	Search(sr *ldap.SearchRequest) (*ldap.SearchResult, error)
}

// Checker ...
type Checker interface {
	Check(Result)
}

// NullChecker ...
type NullChecker struct{}

// Check ...
func (m *NullChecker) Check(Result) {}

// Watch ...
type Watch struct {
	state         int
	searchRequest *ldap.SearchRequest
	monitor       Checker
}

// Result ...
type Result struct {
	Watch   *Watch
	Results *ldap.SearchResult
	Err     error
}

// Watcher watches a set of LDAP nodes, delivering events to a channel.
type Watcher struct {
	conn     Searcher
	logger   *log.Logger
	ticker   *time.Ticker
	duration time.Duration
	watches  []*Watch
	// Events   chan Event
	// Errors   chan error
	// mu       sync.Mutex // Map access
	// fd       int
	// poller   *fdPoller
	// watches  map[string]*watch // Map of inotify watches (key: path)
	// paths    map[int]string    // Map of watched paths (key: watch descriptor)
	// done     chan struct{}     // Channel for sending a "quit message" to the reader goroutine
	// doneResp chan struct{}     // Channel to respond to Close
}

// NewWatcher ...
func NewWatcher(conn Searcher) (*Watcher, error) {
	logger := log.New(os.Stdout, "", log.LstdFlags)

	w := &Watcher{
		conn:     conn,
		duration: 500 * time.Millisecond,
		logger:   logger,
		watches:  make([]*Watch, 0, 10),
		// fd:       fd,
		// poller:   poller,
		// watches:  make(map[string]*watch),
		// paths:    make(map[int]string),
		// Events:   make(chan Event),
		// Errors:   make(chan error),
		// done:     make(chan struct{}),
		// doneResp: make(chan struct{}),
	}

	return w, nil
}

// Start ...
func (w *Watcher) Start() {
	w.logger.Println("initiating watch")

	w.ticker = time.NewTicker(w.duration)

	defer w.ticker.Stop()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	go watch(w)

	<-c
	w.logger.Println("interrupted")
}

// Stop ...
func (w *Watcher) Stop() {
	if w.ticker != nil {
		w.ticker.Stop()
	}
}

// Add ...
func (w *Watcher) Add(sr *ldap.SearchRequest, m Checker) error {
	watch := Watch{state: 0, searchRequest: sr, monitor: m}
	w.watches = append(w.watches, &watch)
	return nil
}

func watch(w *Watcher) {
	for _ = range w.ticker.C {
		w.logger.Println("tick")
		go search(w)
	}
}

func search(w *Watcher) {
	w.logger.Println("searching...")
	for _, watch := range w.watches {
		var result Result
		sr, err := w.conn.Search(watch.searchRequest)

		if err != nil {
			result = Result{Watch: watch, Err: err}
		} else {
			result = Result{Watch: watch, Results: sr}
		}

		watch.monitor.Check(result)
	}
}
