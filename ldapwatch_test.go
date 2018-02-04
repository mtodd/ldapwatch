package ldapwatch

import (
	"flag"
	"fmt"
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

type fakeSearcher struct{}

func (fs fakeSearcher) Search(sr *ldap.SearchRequest) (*ldap.SearchResult, error) {
	return nil, nil
}

func TestNewWatcher(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		var dur time.Duration
		w, _ := NewWatcher(fakeSearcher{}, dur, nil)

		if w.duration != defaultDuration {
			t.Fatalf("default duration (%#v) expected but got %#v", defaultDuration, w.duration)
		}

		if w.logger == nil {
			t.Fatalf("default logger expected but got %#v", w.logger)
		}
	})
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

type testChecker struct {
	prev    Result
	Changed bool
}

func (m *testChecker) Check(r Result) {
	// no previous results (initial search)
	if (Result{}) == m.prev {
		m.prev = r
		return
	}

	// check length differences
	if len(m.prev.Results.Entries) != len(r.Results.Entries) {
		m.Changed = true
		return
	}

	// check DNs match
	prevE := m.prev.Results.Entries[0]
	nextE := r.Results.Entries[0]
	if prevE.DN != nextE.DN {
		m.Changed = true
		return
	}

	// check modifyTimestamp for updates
	prevMod := prevE.GetAttributeValue("modifyTimestamp")
	nextMod := nextE.GetAttributeValue("modifyTimestamp")
	if prevMod != nextMod {
		m.Changed = true
		return
	}

	// fallback comparison when modifyTimestamp won't reflect subsecond updates
	prevMail := prevE.GetAttributeValue("mail")
	nextMail := nextE.GetAttributeValue("mail")
	if prevMail != nextMail {
		m.Changed = true
		return
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
		fmt.Sprintf("(cn=%s)", "copy-Philip J. Fry"),
		[]string{"dn", "mail", "modifyTimestamp"},
		nil,
	)

	t.Run("unmodified", func(t *testing.T) {
		watcher, err := NewWatcher(conn, 500*time.Millisecond, nil)
		if err != nil {
			t.Fatalf("NewWatcher: %s", err)
		}

		mon := &testChecker{}

		watch, err := watcher.Add(searchRequest, mon)
		if err != nil {
			t.Fatalf("Add: %s", err)
		}

		// first run, nothing expected
		watch.tick()

		if mon.Changed {
			t.Fatalf("entry was marked as updated on the first round")
		}

		// second run, nothing expected
		watch.tick()

		if mon.Changed {
			t.Fatalf("entry was marked as updated but should've been unchanged")
		}
	})

	// kill the update gofunc
	t.Run("modified", func(t *testing.T) {
		watcher, err := NewWatcher(conn, 500*time.Millisecond, nil)
		if err != nil {
			t.Fatalf("NewWatcher: %s", err)
		}

		mon := &testChecker{}
		watch, err := watcher.Add(searchRequest, mon)
		if err != nil {
			t.Fatalf("Add: %s", err)
		}

		// first run, nothing expected
		watch.tick()

		if mon.Changed {
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
		watch.tick()

		time.Sleep(100 * time.Millisecond)

		if !mon.Changed {
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
