// users manages accounts in the database from .env (MONGODB_URI,
// MONGODB_DB), like cmd/seed. Start from the backend/ directory:
//
//	go run ./cmd/users add --username tester01 [--display-name Erika] [--role user|admin] [--password-stdin]
//	go run ./cmd/users add-batch --prefix tester --count 10
//	go run ./cmd/users reset-password --username tester01 [--password-stdin]
//	go run ./cmd/users disable --username tester01
//	go run ./cmd/users enable --username tester01
//	go run ./cmd/users revoke-sessions --username tester01
//	go run ./cmd/users revoke-sessions --all
//	go run ./cmd/users list
//	go run ./cmd/users delete --username tester01
//
// Role user gets a generated start password (12 characters) and must change
// it at the first login. Role admin gets a generated password of 24
// characters and no forced change. A generated password is printed exactly
// once; only its bcrypt hash is stored.
//
// A password is never accepted as a command argument (it would end up in
// the shell history). --password-stdin reads one line from standard input
// instead; backend/README.md shows how to type it without echo.
//
// Every successful action writes an entry to the collection auditLog: time,
// action, target username and operator (operating system user and computer
// name). Never passwords or hashes.
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/user"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"lifelog/backend/internal/auth"
	"lifelog/backend/internal/config"
	"lifelog/backend/internal/core"
	"lifelog/backend/internal/store"
)

const usage = `Manage Lifelog accounts in the database from .env.

  go run ./cmd/users add --username NAME [--display-name TEXT] [--role user|admin] [--password-stdin]
  go run ./cmd/users add-batch --prefix PREFIX --count N
  go run ./cmd/users reset-password --username NAME [--password-stdin]
  go run ./cmd/users disable --username NAME
  go run ./cmd/users enable --username NAME
  go run ./cmd/users revoke-sessions (--username NAME | --all)
  go run ./cmd/users list
  go run ./cmd/users delete --username NAME
`

// maxBatch: add-batch creates at most this many accounts at once.
const maxBatch = 100

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	if err := run(os.Args[1], os.Args[2:]); err != nil {
		var ue usageError
		if errors.As(err, &ue) {
			fmt.Fprintf(os.Stderr, "Error: %v\n\n%s", err, usage)
			os.Exit(2)
		}
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

// usageError: wrong command or flags; main prints the usage after it.
type usageError struct{ msg string }

func (e usageError) Error() string { return e.msg }

func run(command string, args []string) error {
	switch command {
	case "add":
		return cmdAdd(args)
	case "add-batch":
		return cmdAddBatch(args)
	case "reset-password":
		return cmdResetPassword(args)
	case "disable":
		return cmdSetDisabled(args, true)
	case "enable":
		return cmdSetDisabled(args, false)
	case "revoke-sessions":
		return cmdRevokeSessions(args)
	case "list":
		return cmdList(args)
	case "delete":
		return cmdDelete(args)
	case "help", "-h", "--help":
		fmt.Print(usage)
		return nil
	}
	return usageError{fmt.Sprintf("unknown command %q", command)}
}

// parseFlags parses args with fs and refuses positional arguments, so a
// password typed after a flag by mistake is not silently ignored.
func parseFlags(fs *flag.FlagSet, args []string) error {
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		return usageError{err.Error()}
	}
	if fs.NArg() > 0 {
		// The argument is not repeated: it might be a password typed by
		// mistake.
		return usageError{"unexpected argument after the flags (passwords are never arguments; use --password-stdin)"}
	}
	return nil
}

// usernameFlag normalizes and checks --username.
func usernameFlag(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", usageError{"--username is required"}
	}
	name := core.NormalizeUsername(value)
	if err := core.CheckUsername(name); err != nil {
		return "", fmt.Errorf("--username %q: %w", value, err)
	}
	return name, nil
}

// parseRole reads --role.
func parseRole(value string) (core.Role, error) {
	switch core.Role(strings.ToLower(strings.TrimSpace(value))) {
	case core.RoleUser:
		return core.RoleUser, nil
	case core.RoleAdmin:
		return core.RoleAdmin, nil
	}
	return "", usageError{fmt.Sprintf("--role must be user or admin, not %q", value)}
}

// mustChangeFor: a user replaces the start password at the first login; an
// admin chose or received a long password and keeps it.
func mustChangeFor(role core.Role) bool {
	return role == core.RoleUser
}

func generatedLength(role core.Role) int {
	if role == core.RoleAdmin {
		return auth.AdminPasswordLength
	}
	return auth.StartPasswordLength
}

// batchUsernames: prefix plus a number with at least two digits, e.g.
// tester01 … tester10. With 100 accounts the numbers get three digits.
func batchUsernames(prefix string, count int) ([]string, error) {
	if count < 1 || count > maxBatch {
		return nil, usageError{fmt.Sprintf("--count must be between 1 and %d", maxBatch)}
	}
	prefix = core.NormalizeUsername(prefix)
	if prefix == "" {
		return nil, usageError{"--prefix is required"}
	}
	width := max(2, len(strconv.Itoa(count)))
	names := make([]string, 0, count)
	for i := 1; i <= count; i++ {
		name := fmt.Sprintf("%s%0*d", prefix, width, i)
		if err := core.CheckUsername(name); err != nil {
			return nil, fmt.Errorf("%q: %w", name, err)
		}
		names = append(names, name)
	}
	return names, nil
}

// maxPasswordLine: a password has at most 72 bytes; the rest of a longer
// line is read only to report the error.
const maxPasswordLine = 1024

// readPassword reads one line from r (without \n or \r\n) and checks the
// password rules. Spaces inside and at the edges count.
func readPassword(r io.Reader) (string, error) {
	line, err := bufio.NewReaderSize(io.LimitReader(r, maxPasswordLine), maxPasswordLine).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read password from standard input: %w", err)
	}
	line = strings.TrimSuffix(line, "\n")
	line = strings.TrimSuffix(line, "\r")
	if err := core.CheckPassword(line); err != nil {
		return "", fmt.Errorf("password from standard input: %w", err)
	}
	return line, nil
}

// passwordFor returns the password to set: from standard input, or newly
// generated (then generated is true and it must be printed once).
func passwordFor(role core.Role, fromStdin bool) (password string, generated bool, err error) {
	if !fromStdin {
		return auth.GeneratePassword(generatedLength(role)), true, nil
	}
	if fi, err := os.Stdin.Stat(); err == nil && fi.Mode()&os.ModeCharDevice != 0 {
		fmt.Fprintln(os.Stderr, "Password (one line; it is visible while typing, see backend/README.md for a hidden prompt):")
	}
	p, err := readPassword(os.Stdin)
	return p, false, err
}

// session bundles what every command needs: the store, the database name
// for the output, the operator for the audit log and a time limit.
type session struct {
	db       *store.Store
	database string
	operator string
	ctx      context.Context
}

func open() (*session, func(), error) {
	if err := config.LoadDotenv(); err != nil {
		return nil, nil, err
	}
	c, err := config.Load(os.Getenv)
	if err != nil {
		return nil, nil, err
	}
	// 60 s: several bcrypt hashes (about 0.25 s each) plus the database
	// over the network.
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	db, err := store.Connect(ctx, c.MongoURI, c.MongoDB)
	if err != nil {
		cancel()
		return nil, nil, err
	}
	closeAll := func() {
		_ = db.Disconnect(context.Background())
		cancel()
	}
	// The unique index on username must exist before the first account.
	if err := db.CreateIndexes(ctx); err != nil {
		closeAll()
		return nil, nil, err
	}
	return &session{db: db, database: c.MongoDB, operator: operator(), ctx: ctx}, closeAll, nil
}

// operator: "<os user>@<computer>" for the audit log.
func operator() string {
	name := "unknown"
	if u, err := user.Current(); err == nil && u.Username != "" {
		name = u.Username
	}
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "unknown"
	}
	return name + "@" + host
}

func (s *session) audit(action, target string) error {
	err := s.db.WriteAudit(s.ctx, core.AuditEntry{
		Time: time.Now(), Action: action, TargetUsername: target, Operator: s.operator,
	})
	if err != nil {
		return fmt.Errorf("done, but the audit log entry failed: %w", err)
	}
	return nil
}

// findUser loads a user or fails with a clear message.
func (s *session) findUser(username string) (core.User, error) {
	u, found, err := s.db.UserByUsername(s.ctx, username)
	if err != nil {
		return core.User{}, err
	}
	if !found {
		return core.User{}, fmt.Errorf("no user %q in database %q", username, s.database)
	}
	return u, nil
}

func cmdAdd(args []string) error {
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	usernameArg := fs.String("username", "", "3 to 32 characters, a-z and 0-9")
	displayArg := fs.String("display-name", "", "name shown in the app (default: the username)")
	roleArg := fs.String("role", "user", "user or admin")
	fromStdin := fs.Bool("password-stdin", false, "read the password from standard input instead of generating one")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	username, err := usernameFlag(*usernameArg)
	if err != nil {
		return err
	}
	role, err := parseRole(*roleArg)
	if err != nil {
		return err
	}
	displayName := username
	if strings.TrimSpace(*displayArg) != "" {
		if displayName, err = core.NormalizeDisplayName(*displayArg); err != nil {
			return fmt.Errorf("--display-name: %w", err)
		}
	}
	password, generated, err := passwordFor(role, *fromStdin)
	if err != nil {
		return err
	}

	s, closeAll, err := open()
	if err != nil {
		return err
	}
	defer closeAll()

	if err := createUser(s, username, displayName, role, password); err != nil {
		return err
	}
	fmt.Printf("User %s created (role %s, database %q).\n", username, role, s.database)
	printPassword(role, password, generated)
	return s.audit(core.AuditAdd, username)
}

func createUser(s *session, username, displayName string, role core.Role, password string) error {
	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}
	err = s.db.CreateUser(s.ctx, core.User{
		ID:                 store.NewID(),
		Username:           username,
		DisplayName:        displayName,
		PasswordHash:       hash,
		Role:               role,
		MustChangePassword: mustChangeFor(role),
		CreatedAt:          time.Now(),
	})
	if errors.Is(err, core.ErrUsernameTaken) {
		return fmt.Errorf("user %q already exists in database %q", username, s.database)
	}
	return err
}

func printPassword(role core.Role, password string, generated bool) {
	if !generated {
		fmt.Println("Password set from standard input.")
	} else if role == core.RoleAdmin {
		fmt.Printf("Password (shown only this once): %s\n", password)
	} else {
		fmt.Printf("Start password (shown only this once): %s\n", password)
	}
	if mustChangeFor(role) {
		fmt.Println("The user must change it at the first login.")
	}
}

func cmdAddBatch(args []string) error {
	fs := flag.NewFlagSet("add-batch", flag.ContinueOnError)
	prefix := fs.String("prefix", "", "start of every username, e.g. tester")
	count := fs.Int("count", 0, fmt.Sprintf("number of accounts (1 to %d)", maxBatch))
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	names, err := batchUsernames(*prefix, *count)
	if err != nil {
		return err
	}

	s, closeAll, err := open()
	if err != nil {
		return err
	}
	defer closeAll()

	// Check every name first, so a batch is not stopped halfway by a name
	// that already exists.
	for _, name := range names {
		if _, found, err := s.db.UserByUsername(s.ctx, name); err != nil {
			return err
		} else if found {
			return fmt.Errorf("user %q already exists in database %q; nothing created", name, s.database)
		}
	}

	type created struct{ name, password string }
	var done []created
	var createErr error
	for _, name := range names {
		password := auth.GeneratePassword(auth.StartPasswordLength)
		if createErr = createUser(s, name, name, core.RoleUser, password); createErr != nil {
			break
		}
		done = append(done, created{name, password})
	}

	// Print what exists, even after an error: these passwords are shown
	// only now.
	if len(done) > 0 {
		fmt.Printf("%d users created (role user, database %q). Start passwords, shown only this once;\n", len(done), s.database)
		fmt.Println("every user must change it at the first login.")
		fmt.Println()
		tw := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(tw, "username\tstart password")
		for _, c := range done {
			fmt.Fprintf(tw, "%s\t%s\n", c.name, c.password)
		}
		tw.Flush()
	}
	var auditErr error
	for _, c := range done {
		if err := s.audit(core.AuditAdd, c.name); err != nil && auditErr == nil {
			auditErr = err
		}
	}
	if createErr != nil {
		return fmt.Errorf("stopped after %d of %d users: %w", len(done), len(names), createErr)
	}
	return auditErr
}

func cmdResetPassword(args []string) error {
	fs := flag.NewFlagSet("reset-password", flag.ContinueOnError)
	usernameArg := fs.String("username", "", "the account")
	fromStdin := fs.Bool("password-stdin", false, "read the new password from standard input instead of generating one")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	username, err := usernameFlag(*usernameArg)
	if err != nil {
		return err
	}

	s, closeAll, err := open()
	if err != nil {
		return err
	}
	defer closeAll()

	u, err := s.findUser(username)
	if err != nil {
		return err
	}
	password, generated, err := passwordFor(u.Role, *fromStdin)
	if err != nil {
		return err
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}
	if found, err := s.db.SetPassword(s.ctx, u.ID, hash, mustChangeFor(u.Role)); err != nil {
		return err
	} else if !found {
		return fmt.Errorf("user %q disappeared", username)
	}
	// After a reset no device stays logged in with the old password.
	ended, err := s.db.DeleteSessionsOfUser(s.ctx, u.ID)
	if err != nil {
		return err
	}
	fmt.Printf("Password of %s reset (role %s, database %q); %d session(s) ended.\n", username, u.Role, s.database, ended)
	printPassword(u.Role, password, generated)
	return s.audit(core.AuditResetPassword, username)
}

func cmdSetDisabled(args []string, disabled bool) error {
	name := "enable"
	if disabled {
		name = "disable"
	}
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	usernameArg := fs.String("username", "", "the account")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	username, err := usernameFlag(*usernameArg)
	if err != nil {
		return err
	}

	s, closeAll, err := open()
	if err != nil {
		return err
	}
	defer closeAll()

	u, err := s.findUser(username)
	if err != nil {
		return err
	}
	if _, err := s.db.SetDisabled(s.ctx, u.ID, disabled); err != nil {
		return err
	}
	if !disabled {
		fmt.Printf("User %s enabled (database %q).\n", username, s.database)
		return s.audit(core.AuditEnable, username)
	}
	// Every request checks disabled anyway; deleting the sessions on top
	// makes the lock immediate and visible.
	ended, err := s.db.DeleteSessionsOfUser(s.ctx, u.ID)
	if err != nil {
		return err
	}
	fmt.Printf("User %s disabled (database %q); %d session(s) ended.\n", username, s.database, ended)
	return s.audit(core.AuditDisable, username)
}

func cmdRevokeSessions(args []string) error {
	fs := flag.NewFlagSet("revoke-sessions", flag.ContinueOnError)
	usernameArg := fs.String("username", "", "end the sessions of this account")
	all := fs.Bool("all", false, "end every session of every account")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if *all == (strings.TrimSpace(*usernameArg) != "") {
		return usageError{"use exactly one of --username and --all"}
	}
	var username string
	if !*all {
		var err error
		if username, err = usernameFlag(*usernameArg); err != nil {
			return err
		}
	}

	s, closeAll, err := open()
	if err != nil {
		return err
	}
	defer closeAll()

	if *all {
		ended, err := s.db.DeleteAllSessions(s.ctx)
		if err != nil {
			return err
		}
		fmt.Printf("%d session(s) of all users ended (database %q).\n", ended, s.database)
		return s.audit(core.AuditRevokeAllSessions, "")
	}
	u, err := s.findUser(username)
	if err != nil {
		return err
	}
	ended, err := s.db.DeleteSessionsOfUser(s.ctx, u.ID)
	if err != nil {
		return err
	}
	fmt.Printf("%d session(s) of %s ended (database %q).\n", ended, username, s.database)
	return s.audit(core.AuditRevokeSessions, username)
}

func cmdList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	s, closeAll, err := open()
	if err != nil {
		return err
	}
	defer closeAll()

	users, err := s.db.ListUsers(s.ctx)
	if err != nil {
		return err
	}
	fmt.Printf("%d user(s) in database %q:\n\n", len(users), s.database)
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	// Never the hash.
	fmt.Fprintln(tw, "username\tdisplayName\trole\tdisabled\tmustChangePassword\tcreatedAt")
	for _, u := range users {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%t\t%t\t%s\n", u.Username, u.DisplayName, u.Role,
			u.Disabled, u.MustChangePassword, u.CreatedAt.UTC().Format(time.RFC3339))
	}
	tw.Flush()
	return s.audit(core.AuditList, "")
}

func cmdDelete(args []string) error {
	fs := flag.NewFlagSet("delete", flag.ContinueOnError)
	usernameArg := fs.String("username", "", "the account")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	username, err := usernameFlag(*usernameArg)
	if err != nil {
		return err
	}

	s, closeAll, err := open()
	if err != nil {
		return err
	}
	defer closeAll()

	u, err := s.findUser(username)
	if err != nil {
		return err
	}
	// Sessions first: if deleting the user fails, nobody is logged in with
	// a half-deleted account.
	ended, err := s.db.DeleteSessionsOfUser(s.ctx, u.ID)
	if err != nil {
		return err
	}
	// TODO(data separation PR): entries have no userId yet. Once they have,
	// delete the user's entries with their chunks, OCR data and originals
	// here too, before the user.
	if _, err := s.db.DeleteUser(s.ctx, u.ID); err != nil {
		return err
	}
	fmt.Printf("User %s deleted with %d session(s) (database %q).\n", username, ended, s.database)
	return s.audit(core.AuditDelete, username)
}
