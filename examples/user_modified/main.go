package main

import (
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/mtodd/ldapwatch"

	ldap "gopkg.in/ldap.v2"
)

// Implements the ldapwatch.Checker interface in order to check whether
// the search results change over time.
//
// In this case, our Checker keeps track of previous results as well as
// holding a channel that we notify whenever changes are detected.
type myChecker struct {
	prev ldapwatch.Result
	c    chan ldapwatch.Result
}

// Check receives the result of the search; the Checker needs to take action
// if the result does not match what it expects.
func (c *myChecker) Check(r ldapwatch.Result) {
	if r.Err != nil {
		log.Printf("%s", r.Err)
		return
	}

	// first search sets baseline
	if (ldapwatch.Result{}) == c.prev {
		c.prev = r
		return
	}

	if len(c.prev.Results.Entries) != len(r.Results.Entries) {
		// entries returned does not match
		c.prev = r
		c.c <- r
		return
	}

	prevEntry := c.prev.Results.Entries[0]
	nextEntry := r.Results.Entries[0]

	if prevEntry.GetAttributeValue("modifyTimestamp") != nextEntry.GetAttributeValue("modifyTimestamp") {
		// modifyTimestamp changed
		c.prev = r
		c.c <- r
		return
	}

	// no change
}

func main() {
	conn, _ := ldap.Dial("tcp", "localhost:389")
	defer conn.Close()
	conn.Bind("cn=admin,dc=planetexpress,dc=com", "GoodNewsEveryone")

	updates := make(chan ldapwatch.Result)
	done := make(chan struct{})
	defer func() { close(done) }()
	go func(c chan ldapwatch.Result, done chan struct{}) {
		for {
			select {
			case result := <-c:
				// result is the search results that have changed
				log.Printf("change detected: %s", result.Results.Entries[0].DN)
				log.Printf("%#v", result)
			case <-done:
				return
			}
		}
	}(updates, done)

	w, err := ldapwatch.NewWatcher(conn, 1*time.Second, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer w.Stop()

	c := myChecker{
		c: updates,
	}

	// Search to monitor for changes
	searchRequest := ldap.NewSearchRequest(
		"ou=people,dc=planetexpress,dc=com",
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		"(cn=Philip J. Fry)",
		[]string{"*", "modifyTimestamp"},
		nil,
	)

	// register the search
	w.Add(searchRequest, &c)

	// run until SIGINT is triggered
	term := make(chan os.Signal, 1)
	signal.Notify(term, os.Interrupt)

	w.Start()

	<-term
}
