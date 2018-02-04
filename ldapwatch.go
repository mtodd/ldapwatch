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
	Check(*ldap.SearchResult, error)
}

// NullChecker ...
type NullChecker struct{}

// Check ...
func (m *NullChecker) Check(*ldap.SearchResult, error) {}

// Watcher coordinates Watch workers.
type Watcher struct {
	conn     Searcher
	logger   *log.Logger
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

	w := Watcher{
		conn:     conn,
		duration: dur,
		logger:   logger,
		watches:  make([]*Watch, 0),
	}

	return &w, nil
}

// Start sets up watch workers and begins working.
func (w *Watcher) Start() {
	for _, watch := range w.watches {
		go watch.start()
		w.wg.Add(1)
	}
}

// Stop ...
func (w *Watcher) Stop() {
	for _, watch := range w.watches {
		watch.stop()
		w.wg.Done()
	}

	w.wg.Wait()
}

// Add instructs the Watcher to periodically check the given search request.
func (w *Watcher) Add(sr *ldap.SearchRequest, c Checker) (Watch, error) {
	watch := Watch{
		watcher:       w,
		searchRequest: sr,
		checker:       c,
		done:          make(chan struct{}),
	}
	w.watches = append(w.watches, &watch)
	return watch, nil
}

// Watch ...
type Watch struct {
	watcher       *Watcher
	searchRequest *ldap.SearchRequest
	checker       Checker
	done          chan struct{}
}

func (w *Watch) start() {
	timer := time.NewTimer(w.watcher.duration)

	for {
		select {
		case <-timer.C:
			w.tick()

			// restart timer
			timer.Reset(w.watcher.duration)
		case <-w.done:
			// halt timer
			timer.Stop()

			w.watcher.logger.Println("finishing")

			return
		}
	}
}

// tell the watch worker to stop work via the done channel
func (w *Watch) stop() {
	w.done <- struct{}{}
}

// perform the search and check the results with the Checker
func (w *Watch) tick() {
	w.checker.Check(w.search())
}

// perform search via the Searcher
func (w *Watch) search() (*ldap.SearchResult, error) {
	return w.watcher.conn.Search(w.searchRequest)
}
