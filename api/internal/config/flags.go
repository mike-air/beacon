package config

// Command-line overrides for the handful of settings you actually change while
// running Beacon locally.
//
// Precedence is flag > environment > default, which is the ordering that makes
// flags worth having: .env stays your baseline and a single argument overrides
// one value for one run, without editing a file you will forget to change back.
//
// Two things this file is deliberate about.
//
// Only a few settings get a flag. Config has around forty fields; a -help
// listing all of them is a wall nobody reads, and most (S3 secret key, SMTP
// password) are things you would never want in shell history or a `ps` listing
// anyway. The ones here are what a second local instance, a scratch database,
// or a quick "run this like production" actually needs.
//
// The two binaries get DIFFERENT flag sets, matching what each one reads.
// beacon-worker never looks at Port, MetricsPort or MeiliURL, so offering it
// -port would be a flag that parses, prints in -help, and does nothing — worse
// than not having it, because it looks like it worked.

import (
	"flag"
	"fmt"
	"io"
	"strings"
)

// LoadAPIFlags reads the environment, applies beacon-api's command-line
// overrides, then validates. args excludes the program name (os.Args[1:]).
//
// Returns flag.ErrHelp when -help was asked for, so the caller can exit 0
// rather than treat a help request as a failure.
func LoadAPIFlags(args []string) (*Config, error) {
	cfg := fromEnv()

	fs := flag.NewFlagSet("beacon-api", flag.ContinueOnError)
	fs.Usage = func() { apiUsage(fs.Output()) }

	// The defaults handed to flag are the values already resolved from the
	// environment, so -help prints what THIS shell would actually run with
	// rather than a generic built-in default.
	port := fs.String("port", cfg.Port, "HTTP listen port")
	databaseURL := fs.String("database-url", cfg.DatabaseURL, "Postgres connection string")
	env := fs.String("env", cfg.Env, `"development" or "production"`)
	redisURL := fs.String("redis-url", cfg.RedisURL, "Redis URL; empty uses the in-process cache only")
	meiliURL := fs.String("meili-url", cfg.MeiliURL, "Meilisearch URL; empty keeps search on Postgres")
	metricsPort := fs.String("metrics-port", cfg.MetricsPort, "internal listener for /metrics and /debug/pprof")

	if err := parse(fs, args); err != nil {
		return nil, err
	}

	applySet(fs, map[string]func(){
		"port":         func() { cfg.Port = *port },
		"database-url": func() { cfg.DatabaseURL = *databaseURL },
		"env":          func() { cfg.Env = *env },
		"redis-url":    func() { cfg.RedisURL = *redisURL },
		"meili-url":    func() { cfg.MeiliURL = *meiliURL },
		"metrics-port": func() { cfg.MetricsPort = *metricsPort },
	})

	if err := cfg.finish(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// LoadWorkerFlags is the same for beacon-worker, whose flags are limited to
// the settings it actually reads.
func LoadWorkerFlags(args []string) (*Config, error) {
	cfg := fromEnv()

	fs := flag.NewFlagSet("beacon-worker", flag.ContinueOnError)
	fs.Usage = func() { workerUsage(fs.Output()) }

	databaseURL := fs.String("database-url", cfg.DatabaseURL, "Postgres connection string")
	env := fs.String("env", cfg.Env, `"development" or "production"`)
	concurrency := fs.Int("concurrency", cfg.WorkerConcurrency, "maximum jobs in flight at once")

	if err := parse(fs, args); err != nil {
		return nil, err
	}

	applySet(fs, map[string]func(){
		"database-url": func() { cfg.DatabaseURL = *databaseURL },
		"env":          func() { cfg.Env = *env },
		"concurrency":  func() { cfg.WorkerConcurrency = *concurrency },
	})

	if err := cfg.finish(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func parse(fs *flag.FlagSet, args []string) error {
	if err := fs.Parse(args); err != nil {
		return err // includes flag.ErrHelp
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected argument %q: there are no subcommands, and flags must come before it", fs.Arg(0))
	}
	return nil
}

// applySet runs the setter for each flag that was ACTUALLY given.
//
// fs.Visit, not fs.VisitAll: Visit walks only the flags present on the command
// line. Today that distinction does not change the result, because every flag
// above is declared with the environment-resolved value as its default, so an
// unset flag assigns cfg.Port = cfg.Port and the two are equivalent. (Checked,
// rather than assumed — swapping in VisitAll leaves every test passing.)
//
// It is Visit anyway because the equivalence depends on that seeding, which is
// the kind of thing a later edit breaks without noticing: declare one flag with
// a literal default — fs.String("port", "8080", …) — and under VisitAll it
// would stamp 8080 over PORT on every run that did not pass -port. Visit is
// correct whatever the defaults are.
func applySet(fs *flag.FlagSet, setters map[string]func()) {
	fs.Visit(func(f *flag.Flag) {
		if set, ok := setters[f.Name]; ok {
			set()
		}
	})
}

// envOnly are the settings with no flag on either binary, listed by -help so
// "how do I set X" is answered on screen rather than in this file.
var envOnly = []string{
	"SHUTDOWN_TIMEOUT", "CORS_ORIGINS", "JWT_SECRET", "JWT_TTL",
	"S3_BUCKET", "S3_REGION", "S3_ENDPOINT", "S3_ACCESS_KEY", "S3_SECRET_KEY",
	"S3_PRESIGN_TTL", "S3_FORCE_PATH_STYLE",
	"SMTP_HOST", "SMTP_PORT", "SMTP_USER", "SMTP_PASS", "EMAIL_FROM",
	"WORKER_POLL_INTERVAL",
	"MEILI_KEY", "MEILI_INDEX",
	"ADMIN_TOKEN", "OTEL_EXPORTER_OTLP_ENDPOINT", "OTEL_TRACE_SAMPLE_RATIO", "BEACON_VERSION",
	"CACHE_TTL",
	"BACKUP_BUCKET", "BACKUP_REGION", "BACKUP_ENDPOINT", "BACKUP_ACCESS_KEY",
	"BACKUP_SECRET_KEY", "BACKUP_AGE_RECIPIENT", "BACKUP_SOURCE_URL",
	"TENANT_RATE_LIMIT_RPS", "TENANT_RATE_LIMIT_BURST",
	"AUTH_RATE_LIMIT_RPS", "AUTH_RATE_LIMIT_BURST",
}

const precedence = `Configuration is read from the environment (and .env, if present). These
flags override it for one run; anything not given keeps its environment
value. Precedence is flag > environment > default.

`

func apiUsage(w io.Writer) {
	fmt.Fprint(w, "beacon-api — the Beacon HTTP API\n\nUsage:\n  beacon-api [flags]\n\n"+precedence+
		`Flags:
  -port            HTTP listen port
  -database-url    Postgres connection string
  -env             "development" or "production"
  -redis-url       Redis URL; empty uses the in-process cache only
  -meili-url       Meilisearch URL; empty keeps search on Postgres
  -metrics-port    internal listener for /metrics and /debug/pprof
  -help            this message

Examples:
  # a second instance beside one already running
  beacon-api -port 8081 -metrics-port 9091

  # a scratch database, without editing .env
  beacon-api -database-url postgres://beacon:beacon@localhost:5432/scratch?sslmode=disable

  # exercise the production paths locally (JWT_SECRET becomes required)
  beacon-api -env production

`)
	writeEnvOnly(w)
}

func workerUsage(w io.Writer) {
	fmt.Fprint(w, "beacon-worker — the Beacon background job runner\n\nUsage:\n  beacon-worker [flags]\n\n"+precedence+
		`Flags:
  -database-url    Postgres connection string
  -env             "development" or "production"
  -concurrency     maximum jobs in flight at once
  -help            this message

The worker reads no HTTP or search settings, so it has no -port, -meili-url
or -metrics-port: a flag that parsed and then did nothing would be worse
than its absence.

Examples:
  # watch one job at a time, so the log is readable
  beacon-worker -concurrency 1

`)
	writeEnvOnly(w)
}

func writeEnvOnly(w io.Writer) {
	fmt.Fprint(w, "Set by environment only:\n")
	// Wrapped rather than one per line: this is a reference list, and forty
	// lines of it would push the flags themselves off the screen.
	line := " "
	for _, name := range envOnly {
		if len(line)+len(name)+1 > 76 {
			fmt.Fprintln(w, line)
			line = " "
		}
		line += " " + name
	}
	if strings.TrimSpace(line) != "" {
		fmt.Fprintln(w, line)
	}
	fmt.Fprint(w, "\nSee .env.example for what each one means.\n")
}
