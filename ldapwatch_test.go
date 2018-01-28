package ldapwatch

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"testing"

	ldap "gopkg.in/ldap.v2"
)

const (
	defaultHost = "localhost"
	defaultPort = 389

	bindusername = "cn=admin,dc=planetexpress,dc=com"
	bindpassword = "GoodNewsEveryone"

	base = "ou=people,dc=planetexpress,dc=com"
)

var (
	network string
)

func TestMain(m *testing.M) {
	host := os.Getenv("LDAPHOST")
	if host == "" {
		host = "localhost"
	}

	port, err := strconv.Atoi(os.Getenv("LDAPPORT"))
	if err != nil {
		port = defaultPort
	}

	network = fmt.Sprintf("%s:%d", host, port)

	flag.Parse()
	os.Exit(m.Run())
}

func TestEnv(t *testing.T) {
	conn, err := ldap.Dial("tcp", network)
	if err != nil {
		t.Fatalf("ldap.Dial: %s", err)
	}
	defer conn.Close()

	err = conn.Bind(bindusername, bindpassword)
	if err != nil {
		t.Fatalf("ldap.Bind: %s", err)
	}
}

func TestWatchPerson(t *testing.T) {
	conn, err := ldap.Dial("tcp", network)
	if err != nil {
		t.Fatalf("ldap.Dial: %s", err)
	}
	defer conn.Close()

	err = conn.Bind(bindusername, bindpassword)
	if err != nil {
		t.Fatalf("ldap.Bind: %s", err)
	}

	// Search for the given username
	searchRequest := ldap.NewSearchRequest(
		base,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		// fmt.Sprintf("(&(objectClass=organizationalPerson)&(uid=%s))", username),
		fmt.Sprintf("(uid=%s)", "fry"),
		[]string{"dn", "modifyTimestamp"},
		nil,
	)

	// FIXME: duplicate a person that we can modify

	updated := false
	updates := make(chan Result)
	done := make(chan struct{})
	defer func() { close(done) }()
	go func(c chan Result, done chan struct{}) {
		for {
			select {
			case <-c:
				updated = true
			case <-done:
				return
			}
		}
	}(updates, done)

	compare := func(prev Result, next Result) bool {
		// no previous results (initial search); treat as unchanged
		if (Result{}) == prev {
			return false
		}

		// check length differences
		if len(prev.Results.Entries) != len(next.Results.Entries) {
			return true
		}

		prevE := prev.Results.Entries[0]
		nextE := next.Results.Entries[0]
		if prevE.DN != nextE.DN {
			// log.Println(fmt.Sprintf("DN mismatch %#v %#v", prevE, nextE))
			return true
		}

		prevMod := nextE.GetAttributeValue("modified")
		nextMod := nextE.GetAttributeValue("modified")
		if prevMod != nextMod {
			// log.Println(fmt.Sprintf("modified mismatch %#v %#v", prevMod, nextMod))
			return true
		}

		return false
	}

	t.Run("unmodified", func(t *testing.T) {
		updated = false

		watcher, err := NewWatcher(conn)
		if err != nil {
			t.Fatalf("NewWatcher: %s", err)
		}

		err = watcher.Add(searchRequest, compare, updates)
		if err != nil {
			log.Fatal(err)
		}

		// first run, nothing expected
		search(watcher)

		if updated {
			t.Fatalf("entry was marked as updated on the first round")
		}

		// second run, nothing expected
		search(watcher)

		if updated {
			t.Fatalf("entry was marked as updated but should've been unchanged")
		}
	})

	// kill the update gofunc
	t.Run("modified", func(t *testing.T) {
		updated = false

		watcher, err := NewWatcher(conn)
		if err != nil {
			t.Fatalf("NewWatcher: %s", err)
		}

		err = watcher.Add(searchRequest, compare, updates)
		if err != nil {
			log.Fatal(err)
		}

		// first run, nothing expected
		search(watcher)

		if updated {
			t.Fatalf("entry was marked as updated on the first round")
		}

		// second run, nothing expected
		search(watcher)

		if !updated {
			t.Fatalf("entry was not marked as updated but should've been")
		}
	})
}

func TestWatchPeople(t *testing.T) {
	// t.Errorf("because")
}

func TestWatchGroup(t *testing.T) {
	// t.Errorf("because")
}
