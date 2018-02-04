package ldapwatch

import (
	"log"
	"os"
	"sync"
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

// Result ...
type Result struct {
	Watch   *Watch
	Results *ldap.SearchResult
	Err     error
}

// Watcher coordinates Watch workers.
type Watcher struct {
	conn     Searcher
	logger   *log.Logger
	ticker   *time.Ticker
	duration time.Duration
	watches  []*Watch
	wg       sync.WaitGroup
}

const defaultDuration = 500 * time.Millisecond

// NewWatcher constructs a Watcher.
func NewWatcher(conn Searcher, dur time.Duration, logger *log.Logger) (*Watcher, error) {
	if dur == 0 {
		dur = defaultDuration
	}

	if logger == nil {
		logger = log.New(os.Stdout, "", log.LstdFlags)
	}

	w := &Watcher{
		conn:     conn,
		duration: dur,
		logger:   logger,
		watches:  make([]*Watch, 0),
	}

	return w, nil
}

// Start ...
func (w *Watcher) Start() {
	w.ticker = time.NewTicker(w.duration)
	for _, watch := range w.watches {
		go func(watch *Watch) {
			w.logger.Println("worker started")
			for {
				select {
				case <-watch.tick:
					watch.Tick()
				case <-watch.done:
					w.logger.Println("finishing")
					w.wg.Done()
					return
				}
			}
		}(watch)
		w.wg.Add(1)
	}

	// await ticks, fanout ticks to workers
	go fanoutTicks(w)
}

// Stop ...
func (w *Watcher) Stop() {
	if w.ticker != nil {
		w.ticker.Stop()
	}

	for _, watch := range w.watches {
		watch.done <- struct{}{}
	}

	w.wg.Wait()
}

// Watch ...
type Watch struct {
	watcher       *Watcher
	searchRequest *ldap.SearchRequest
	checker       Checker
	tick          chan struct{}
	done          chan struct{}
}

// Add instructs the Watcher to periodically check the given search request.
func (w *Watcher) Add(sr *ldap.SearchRequest, c Checker) (Watch, error) {
	watch := Watch{
		watcher:       w,
		searchRequest: sr,
		checker:       c,
		tick:          make(chan struct{}),
		done:          make(chan struct{}),
	}
	w.watches = append(w.watches, &watch)
	return watch, nil
}

func fanoutTicks(w *Watcher) {
	for _ = range w.ticker.C {
		for _, watch := range w.watches {
			watch.tick <- struct{}{}
		}
	}
}

func (w *Watch) Tick() error {
	var result Result
	sr, err := w.watcher.conn.Search(w.searchRequest)

	if err != nil {
		result = Result{Watch: w, Err: err}
	} else {
		result = Result{Watch: w, Results: sr}
	}

	w.checker.Check(result)

	return nil
}
