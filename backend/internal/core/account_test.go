package core

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestNormalizeUsername(t *testing.T) {
	cases := map[string]string{
		"tester01":     "tester01",
		"Tester01":     "tester01",
		"  TESTER01 ":  "tester01",
		"\ttester01\n": "tester01",
		"":             "",
	}
	for in, want := range cases {
		if got := NormalizeUsername(in); got != want {
			t.Errorf("NormalizeUsername(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCheckUsername(t *testing.T) {
	cases := []struct {
		username string
		ok       bool
	}{
		{"abc", true},
		{"tester01", true},
		{"0123456789", true},
		{strings.Repeat("a", 32), true},
		{"ab", false},
		{strings.Repeat("a", 33), false},
		{"", false},
		{"Tester01", false}, // stored form is lowercase
		{"tester-01", false},
		{"tester_01", false},
		{"tester 01", false},
		{" tester01", false},
		{"müller", false},
		{"tester01\n", false},
	}
	for _, c := range cases {
		err := CheckUsername(c.username)
		if c.ok && err != nil {
			t.Errorf("CheckUsername(%q) = %v, want nil", c.username, err)
		}
		if !c.ok && !errors.Is(err, ErrInvalidUsername) {
			t.Errorf("CheckUsername(%q) = %v, want ErrInvalidUsername", c.username, err)
		}
	}
}

func TestCheckPassword(t *testing.T) {
	cases := []struct {
		name     string
		password string
		problem  PasswordProblem // "" = valid
	}{
		{"10 characters", "abcdefghij", ""},
		{"9 characters", "abcdefghi", PasswordTooShort},
		{"empty", "", PasswordTooShort},
		{"spaces count", "          ", ""},
		{"72 bytes", strings.Repeat("a", 72), ""},
		{"73 bytes", strings.Repeat("a", 73), PasswordTooLong},
		// An umlaut has 2 bytes in UTF-8: 10 characters are enough, 36 fit
		// into 72 bytes, 37 do not.
		{"10 umlauts", strings.Repeat("ä", 10), ""},
		{"36 umlauts", strings.Repeat("ä", 36), ""},
		{"37 umlauts", strings.Repeat("ä", 37), PasswordTooLong},
		{"9 umlauts (18 bytes)", strings.Repeat("ä", 9), PasswordTooShort},
		{"emoji count as one character", strings.Repeat("🔑", 9) + "a", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := CheckPassword(c.password)
			checkProblem(t, err, c.problem)
		})
	}
}

func TestCheckNewPassword(t *testing.T) {
	checkProblem(t, CheckNewPassword("altesPasswort1", "neuesPasswort1"), "")
	checkProblem(t, CheckNewPassword("altesPasswort1", "altesPasswort1"), PasswordUnchanged)
	checkProblem(t, CheckNewPassword("altesPasswort1", "kurz"), PasswordTooShort)
	checkProblem(t, CheckNewPassword("altesPasswort1", strings.Repeat("x", 73)), PasswordTooLong)
}

func checkProblem(t *testing.T, err error, want PasswordProblem) {
	t.Helper()
	if want == "" {
		if err != nil {
			t.Errorf("err = %v, want nil", err)
		}
		return
	}
	var we *WeakPasswordError
	if !errors.As(err, &we) {
		t.Fatalf("err = %v, want WeakPasswordError", err)
	}
	if we.Problem != want {
		t.Errorf("problem = %q, want %q", we.Problem, want)
	}
}

func TestNormalizeDisplayName(t *testing.T) {
	cases := []struct {
		in, want string
		ok       bool
	}{
		{"Erika", "Erika", true},
		{"  Erika Muster  ", "Erika Muster", true},
		{"Ö", "Ö", true},
		{strings.Repeat("ä", 50), strings.Repeat("ä", 50), true},
		{strings.Repeat("ä", 51), "", false},
		{"👩‍⚕️ Erika", "👩‍⚕️ Erika", true},
		{"", "", false},
		{"   ", "", false},
		{"Erika\nMuster", "", false},
		{"Erika\tMuster", "", false},
		{"Erika\x00", "", false},
		{"Erika\x7f", "", false},
		{"Erika\xff", "", false}, // invalid UTF-8
	}
	for _, c := range cases {
		got, err := NormalizeDisplayName(c.in)
		if c.ok {
			if err != nil || got != c.want {
				t.Errorf("NormalizeDisplayName(%q) = %q, %v; want %q", c.in, got, err, c.want)
			}
			continue
		}
		if !errors.Is(err, ErrInvalidDisplayName) {
			t.Errorf("NormalizeDisplayName(%q) = %q, %v; want ErrInvalidDisplayName", c.in, got, err)
		}
	}
}

func TestRetryAfterSeconds(t *testing.T) {
	cases := map[time.Duration]int{
		time.Minute:                       60,
		59*time.Second + time.Millisecond: 60, // rounded up
		time.Millisecond:                  1,
		0:                                 1, // never 0: "try again now" would be wrong while locked
		15 * time.Minute:                  900,
	}
	for d, want := range cases {
		e := &TooManyAttemptsError{RetryAfter: d}
		if got := e.RetryAfterSeconds(); got != want {
			t.Errorf("RetryAfterSeconds(%v) = %d, want %d", d, got, want)
		}
	}
}
