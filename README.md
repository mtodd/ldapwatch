## Usage

``` go
package main

import (
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
  // first search sets baseline
  if (ldapwatch.Result{}) == c.prev {
    c.prev = r
    return
  }

  if len(c.prev.Entries) != len(r.Entries) {
    // entries returned does not match
    c.c <- r
    return
  }

  prevEntry := c.prev.Entries[0]
  nextEntry := r.Entries[0]

  if prevEntry.GetAttributeValue("modifyTimestamp") != nextEntry.GetAttributeValue("modifyTimestamp") {
    // modifyTimestamp changed
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
  go func(c chan Result, done chan struct{}) {
    for {
      select {
      case result := <-c:
        // result is the search results that have changed
      case <-done:
        return
      }
    }
  }(updates, done)

  w := ldapwatch.NewWatcher(conn)

  c := &myChecker{updates}

  // Search to monitor for changes
  searchRequest := ldap.NewSearchRequest(
    "cn=Philip J. Fry,ou=people,dc=planetexpress,dc=com",
    ldap.ScopeBaseObject, ldap.NeverDerefAliases, 0, 0, false,
    "",
    []string{"*", "modifyTimestamp"},
    , nil,
  )

  // register the search
  w.Add(searchRequest, c)

  // run until SIGINT is triggered
  w.Start()
}
```

## Development & Testing Environment

We use the following Docker container to run our testing OpenLDAP service:
https://store.docker.com/community/images/rroemhild/test-openldap

``` shell
docker-compose build
docker-compose run ldapwatch
```
