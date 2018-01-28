package ldapwatch

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

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

	updated bool
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

func findEntry(c *ldap.Conn, cn string) (*ldap.Entry, error) {
	sr := ldap.NewSearchRequest(
		base,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		fmt.Sprintf("(cn=%s)", cn),
		nil,
		nil,
	)
	res, err := c.Search(sr)
	if err != nil {
		return nil, err
	}

	if len(res.Entries) != 1 {
		return nil, fmt.Errorf("one result expected, %d found", len(res.Entries))
	}

	return res.Entries[0], nil
}

func clearDupes(c *ldap.Conn, dupeRdn string) error {
	sr := ldap.NewSearchRequest(
		base,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		fmt.Sprintf("(cn=copy-%s)", dupeRdn),
		nil,
		nil,
	)
	res, err := c.Search(sr)
	if err != nil {
		return err
	}

	if len(res.Entries) > 0 {
		for _, entry := range res.Entries {
			dr := ldap.NewDelRequest(entry.DN, nil)
			if err := c.Del(dr); err != nil {
				return err
			}
		}
	}

	return nil
}

func dupEntry(c *ldap.Conn, existingRdn string) error {
	entry, err := findEntry(c, existingRdn)
	if err != nil {
		return err
	}

	dupeDn := strings.Replace(entry.DN, existingRdn, "copy-"+existingRdn, 1)
	ar := ldap.NewAddRequest(dupeDn)

	// copy attributes
	for _, entryAttr := range entry.Attributes {
		switch entryAttr.Name {
		case "uid":
			fallthrough
		case "cn":
			ar.Attribute(entryAttr.Name, []string{"copy-" + entry.GetAttributeValue(entryAttr.Name)})
		default:
			ar.Attribute(entryAttr.Name, entryAttr.Values)
		}
	}

	return c.Add(ar)
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

	if err = clearDupes(conn, "Philip J. Fry"); err != nil {
		t.Fatalf("clearDupes: %s", err)
	}
	if err = dupEntry(conn, "Philip J. Fry"); err != nil {
		t.Fatalf("dupEntry: %s", err)
	}

	// Search for the given username
	searchRequest := ldap.NewSearchRequest(
		base,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		// fmt.Sprintf("(&(objectClass=organizationalPerson)&(uid=%s))", username),
		fmt.Sprintf("(cn=%s)", "copy-Philip J. Fry"),
		[]string{"dn", "mail", "modifyTimestamp"},
		nil,
	)

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

		prevMod := prevE.GetAttributeValue("modifyTimestamp")
		nextMod := nextE.GetAttributeValue("modifyTimestamp")
		if prevMod != nextMod {
			// log.Println(fmt.Sprintf("modifyTimestamp mismatch %#v %#v", prevMod, nextMod))
			return true
		}

		// fallback comparison when modifyTimestamp won't reflect subsecond updates
		prevMail := prevE.GetAttributeValue("mail")
		nextMail := nextE.GetAttributeValue("mail")
		if prevMail != nextMail {
			// log.Println(fmt.Sprintf("mail mismatch %#v %#v", prevMail, nextMail))
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

		time.Sleep(1 * time.Second)

		entry, err := findEntry(conn, "copy-Philip J. Fry")
		if err != nil {
			t.Fatalf("%s", err)
		}
		mr := ldap.NewModifyRequest(entry.DN)
		mr.Replace("mail", []string{"fired@example.org"})
		if err = conn.Modify(mr); err != nil {
			t.Fatalf("modify failed: %s", err)
		}

		// second run, nothing expected
		search(watcher)

		time.Sleep(100 * time.Millisecond)

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
