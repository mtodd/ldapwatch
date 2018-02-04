# `ldapwatch`

A library for watching LDAP.

## Usage

Connect to LDAP (error checking elided for brevity):

``` go
conn, _ := ldap.Dial("tcp", "localhost:389")
defer conn.Close()
conn.Bind("cn=admin,dc=planetexpress,dc=com", "GoodNewsEveryone")
```

Construct an `ldap.Watcher`:

``` go
w, err := ldapwatch.NewWatcher(conn, 30 * time.Second, nil)
```

Define an LDAP search to watch:

``` go
searchRequest := ldap.NewSearchRequest(
  "ou=people,dc=planetexpress,dc=com",
  ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
  "(cn=Philip J. Fry)",
  []string{"*", "modifyTimestamp"},
  nil,
)
```

Implement the `ldapwatch.Checker` interface for your check. For example:

``` go
type myChecker struct {
	prev ldapwatch.Result
}

func (c *myChecker) Check(r ldapwatch.Result) {
	// first search sets baseline
	if (ldapwatch.Result{}) == c.prev {
		c.prev = r
		return
	}

	prevEntry := c.prev.Results.Entries[0]
	nextEntry := r.Results.Entries[0]

	if prevEntry.GetAttributeValue("modifyTimestamp") != nextEntry.GetAttributeValue("modifyTimestamp") {
		// modifyTimestamp changed

    // update to current results
		c.prev = r

    // handle change event
    log.Printf("user updated: %s", nextEntry.DN)

		return
	}

	// no change
}
```

NOTE: Error handling elided for brevity. How and what is checked for and what is done must be defined (by you) in your `ldapwatch.Checker`.

Tell the `ldapwatch.Watcher` to use the `ldapwatch.Checker` to watch this `searchRequest`:

``` go
w.Add(searchRequest, myChecker{})
```

Start watching until `w.Stop()` is called (deferred in this case):

``` go
defer w.Stop()
w.Start()
```

NOTE: `Start()` is nonblocking. You are responsible for sleeping/waiting your calling goroutine (usually `main()`) while the `ldapwatch.Watcher` works.

### Example

Check out the [example user updated check](./examples/user_modified/main.go) for a working reference implementation.

## Development & Testing Environment

We use the following Docker container to run our testing OpenLDAP service:
https://store.docker.com/community/images/rroemhild/test-openldap

``` shell
docker-compose build
docker-compose run ldapwatch
```

NOTE: Once the `ldapwatch_ldap` container is running, you can run tests with:

``` shell
make test
```
