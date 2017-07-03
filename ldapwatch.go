package ldapwatch

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	ldap "gopkg.in/ldap.v2"
)

// Watch ...
type Watch struct {
	state int
}

// Watcher watches a set of LDAP nodes, delivering events to a channel.
type Watcher struct {
	conn     *ldap.Conn
	logger   *log.Logger
	ticker   *time.Ticker
	duration time.Duration
	watches  map[string]Watch
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
func NewWatcher() (*Watcher, error) {
	logger := log.New(os.Stdout, "", log.LstdFlags)

	w := &Watcher{
		duration: 500 * time.Millisecond,
		logger:   logger,
		watches:  make(map[string]Watch),
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

// Connect ...
func (w *Watcher) Connect(host string, port int) {
	var err error
	w.conn, err = ldap.Dial("tcp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		w.logger.Fatal(err)
	}
}

// Bind ...
func (w *Watcher) Bind(user string, password string) {
	err := w.conn.Bind(user, password)
	if err != nil {
		w.logger.Fatal(err)
	}
}

// Start ...
func (w *Watcher) Start() {
	w.logger.Println("initiating watch")

	w.ticker = time.NewTicker(w.duration)

	defer w.ticker.Stop()
	defer w.conn.Close()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	go watch(w)

	<-c
	w.logger.Println("interrupted")
}

// Stop ...
func (w *Watcher) Stop() {
	w.ticker.Stop()
}

// Add ...
func (w *Watcher) Add(dn string) error {
	// w.logger.Println(fmt.Sprintf("watching %s", dn))
	w.watches[dn] = Watch{state: 0}
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
	for dn, watch := range w.watches {
		w.logger.Println(dn)

		// Search for the given username
		searchRequest := ldap.NewSearchRequest(
			dn,
			ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
			// fmt.Sprintf("(&(objectClass=organizationalPerson)&(uid=%s))", username),
			"(objectClass=*)",
			[]string{"dn"},
			nil,
		)

		sr, err := w.conn.Search(searchRequest)
		if err != nil {
			w.logger.Println(err)
			watch.state = -1
		}

		if len(sr.Entries) != 1 {
			w.logger.Println(fmt.Sprintf("%s not found", dn))
			watch.state = 1
		}

		userdn := sr.Entries[0].DN
		w.logger.Println(fmt.Sprintf("%s found", userdn))
		watch.state = 0
	}
}
