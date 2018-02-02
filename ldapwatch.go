package ldapwatch

import (
	"log"
	"os"
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
	checker       Checker
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
}

// NewWatcher ...
func NewWatcher(conn Searcher) (*Watcher, error) {
	logger := log.New(os.Stdout, "", log.LstdFlags)

	w := &Watcher{
		conn:     conn,
		duration: 500 * time.Millisecond,
		logger:   logger,
		watches:  make([]*Watch, 0, 10),
	}

	return w, nil
}

// Start ...
func (w *Watcher) Start() {
	w.ticker = time.NewTicker(w.duration)

	go watch(w)
}

// Stop ...
func (w *Watcher) Stop() {
	if w.ticker != nil {
		w.ticker.Stop()
	}
}

// Add ...
func (w *Watcher) Add(sr *ldap.SearchRequest, c Checker) error {
	watch := Watch{state: 0, searchRequest: sr, checker: c}
	w.watches = append(w.watches, &watch)
	return nil
}

func watch(w *Watcher) {
	for _ = range w.ticker.C {
		go tick(w)
	}
}

func tick(w *Watcher) {
	w.logger.Println("searching...")
	for _, watch := range w.watches {
		var result Result
		sr, err := w.conn.Search(watch.searchRequest)

		if err != nil {
			result = Result{Watch: watch, Err: err}
		} else {
			result = Result{Watch: watch, Results: sr}
		}

		watch.checker.Check(result)
	}
}
