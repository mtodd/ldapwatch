## Usage

``` go
package main

import (
  "github.com/mtodd/ldapwatch"

  ldap "gopkg.in/ldap.v2"
)

func main() {
  conn, _ := ldap.Dial("tcp", "localhost:389")
  defer conn.Close()
  conn.Bind("cn=admin,dc=planetexpress,dc=com", "GoodNewsEveryone")

  w := ldapwatch.NewWatcher(conn)
  compare := func(before, after ldapwatch.Result) bool {
    if len(before.Entries) != len(after.Entries) {
      // entries returned does not match
      return true
    }

    beforeEntry := before.Entries[0]
    afterEntry := after.Entries[0]

    if beforeEntry.GetAttributeValue("modifyTimestamp") != afterEntry.GetAttributeValue("modifyTimestamp") {
      // modifyTimestamp changed
      return true
    }

    // no change
    return false
  }

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

  // Search to monitor for changes
  searchRequest := ldap.NewSearchRequest(
    "cn=Philip J. Fry,ou=people,dc=planetexpress,dc=com",
    ldap.ScopeBaseObject, ldap.NeverDerefAliases, 0, 0, false,
    "",
    []string{"*", "modifyTimestamp"},
    , nil,
  )

  // register the search
  w.Add(searchRequest, compare, updates)

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
