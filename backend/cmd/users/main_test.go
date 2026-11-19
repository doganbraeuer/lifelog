package main

import (
	"errors"
	"flag"
	"strings"
	"testing"

	"lifelog/backend/internal/core"
)

func TestBatchUsernames(t *testing.T) {
	names, err := batchUsernames("tester", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 10 || names[0] != "tester01" || names[9] != "tester10" {
		t.Errorf("names = %v", names)
	}

	names, _ = batchUsernames("Tester", 3) // prefix is lowercased
	if strings.Join(names, ",") != "tester01,tester02,tester03" {
		t.Errorf("names = %v", names)
	}

	names, _ = batchUsernames("t", 100) // three digits from 100 on
	if names[0] != "t001" || names[99] != "t100" {
		t.Errorf("first/last = %s/%s", names[0], names[99])
	}
}

func TestBatchUsernames_Invalid(t *testing.T) {
	for _, c := range []struct {
		prefix string
		count  int
	}{
		{"tester", 0},
		{"tester", 101},
		{"tester", -1},
		{"", 5},
		{"test-er", 2},               // not a-z, 0-9
		{strings.Repeat("a", 31), 2}, // a31 + 2 digits = 33 characters
	} {
		if _, err := batchUsernames(c.prefix, c.count); err == nil {
			t.Errorf("%q/%d: want an error", c.prefix, c.count)
		}
	}
	// The shortest valid case: "t01" has exactly 3 characters.
	if names, err := batchUsernames("t", 1); err != nil || names[0] != "t01" {
		t.Errorf("t/1 = %v, %v; want [t01]", names, err)
	}
}

func TestParseRole(t *testing.T) {
	for in, want := range map[string]core.Role{"user": core.RoleUser, "admin": core.RoleAdmin, " Admin ": core.RoleAdmin} {
		if got, err := parseRole(in); err != nil || got != want {
			t.Errorf("parseRole(%q) = %q, %v", in, got, err)
		}
	}
	for _, in := range []string{"", "root", "administrator"} {
		if _, err := parseRole(in); err == nil {
			t.Errorf("parseRole(%q): want an error", in)
		}
	}
}

func TestRoleDefaults(t *testing.T) {
	if !mustChangeFor(core.RoleUser) || mustChangeFor(core.RoleAdmin) {
		t.Error("user must change the start password, admin not")
	}
	if generatedLength(core.RoleUser) != 12 || generatedLength(core.RoleAdmin) != 24 {
		t.Error("generated lengths: user 12, admin 24")
	}
}

func TestReadPassword(t *testing.T) {
	cases := []struct {
		in, want string
		ok       bool
	}{
		{"Sonnenblume am Mönchsberg\n", "Sonnenblume am Mönchsberg", true},
		{"Sonnenblume am Mönchsberg\r\n", "Sonnenblume am Mönchsberg", true}, // Windows line ending
		{"Sonnenblume am Mönchsberg", "Sonnenblume am Mönchsberg", true},     // no line ending
		{"  mit Leerzeichen  \n", "  mit Leerzeichen  ", true},               // spaces count
		{"erste Zeile ist es\nzweite Zeile\n", "erste Zeile ist es", true},
		{"kurz\n", "", false},
		{"\n", "", false},
		{"", "", false},
		{strings.Repeat("a", 73) + "\n", "", false},
		{strings.Repeat("a", 5000), "", false},
	}
	for _, c := range cases {
		got, err := readPassword(strings.NewReader(c.in))
		if c.ok && (err != nil || got != c.want) {
			t.Errorf("readPassword(%q) = %q, %v; want %q", c.in, got, err, c.want)
		}
		if !c.ok {
			var we *core.WeakPasswordError
			if !errors.As(err, &we) {
				t.Errorf("readPassword(%q) = %q, %v; want WeakPasswordError", c.in, got, err)
			}
		}
	}
}

func TestUsernameFlag(t *testing.T) {
	if got, err := usernameFlag(" Tester01 "); err != nil || got != "tester01" {
		t.Errorf("usernameFlag = %q, %v", got, err)
	}
	if _, err := usernameFlag(""); !errors.As(err, new(usageError)) {
		t.Errorf("empty: err = %v, want usageError", err)
	}
	if _, err := usernameFlag("ab"); !errors.Is(err, core.ErrInvalidUsername) {
		t.Errorf("too short: err = %v, want ErrInvalidUsername", err)
	}
}

// A stray positional argument (for example a password typed after a flag)
// is refused instead of ignored.
func TestParseFlags_RefusesPositionalArguments(t *testing.T) {
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	fs.String("username", "", "")
	err := parseFlags(fs, []string{"--username", "tester01", "MeinPasswort123"})
	if err == nil || !strings.Contains(err.Error(), "--password-stdin") {
		t.Fatalf("err = %v, want a hint to --password-stdin", err)
	}
	if strings.Contains(err.Error(), "MeinPasswort123") {
		t.Error("the error must not repeat the argument, it might be a password")
	}
}

func TestRun_UnknownCommand(t *testing.T) {
	if err := run("remove", nil); !errors.As(err, new(usageError)) {
		t.Errorf("err = %v, want usageError", err)
	}
}
