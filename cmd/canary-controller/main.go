// Command canary-controller walks a deploy up the rollout ladder, checking the
// gate at every rung, and puts traffic back to zero the moment anything looks
// wrong. Chapter 47.
//
// The ladder — 1%, 10%, 50%, 100% — is not arbitrary. Each rung catches a
// different class of bug at a blast radius you chose in advance:
//
//	1%    a crash on startup, a missing env var, an obvious 500
//	10%   an error that needs a bit of traffic to show up at all
//	50%   resource problems: connection pool, memory, a slow query under load
//	100%  everyone
//
// Four signals decide whether to climb or stop:
//
//	error rate vs baseline    the obvious one
//	p95 latency vs baseline   slow is a failure too, it just takes longer
//	SLO budget consumed       "still under the threshold" is not the same as
//	                          "not eating this month's error budget"
//	new error fingerprints    an error signature nobody has ever seen before is
//	                          almost always a regression, even at a tiny rate
//
// [verbatim ch47] with the flip, the promotion and the Prometheus plumbing the
// chapter's checkGate implies filled in.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	promapi "github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

// CanaryStep is one rung of the rollout ladder.
type CanaryStep struct {
	Percent int
	HoldFor time.Duration
}

var rollout = []CanaryStep{
	{Percent: 1, HoldFor: 10 * time.Minute},
	{Percent: 10, HoldFor: 20 * time.Minute},
	{Percent: 50, HoldFor: 30 * time.Minute},
	{Percent: 100, HoldFor: 0},
}

func main() {
	var (
		promURL     = flag.String("prometheus", envOr("PROMETHEUS_URL", "http://localhost:9091"), "Prometheus base URL")
		canaryColor = flag.String("color", "", "the canary stack's color: blue or green")
		dryRun      = flag.Bool("dry-run", false, "check the gate once and exit, changing nothing")
	)
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if *canaryColor != "blue" && *canaryColor != "green" {
		log.Error("--color must be blue or green")
		os.Exit(2)
	}

	if err := run(context.Background(), log, *promURL, *canaryColor, *dryRun); err != nil {
		log.Error("canary aborted", "err", err)
		// Non-zero exit is the contract with CI: a failed canary must fail the
		// pipeline, not print a warning nobody reads.
		os.Exit(1)
	}
}

func run(ctx context.Context, log *slog.Logger, promURL, canaryColor string, dryRun bool) error {
	client, err := promapi.NewClient(promapi.Config{Address: promURL})
	if err != nil {
		return fmt.Errorf("prometheus client: %w", err)
	}
	prom := promv1.NewAPI(client)

	if dryRun {
		if err := checkGate(ctx, prom, canaryColor); err != nil {
			return err
		}
		log.Info("gate passes", "color", canaryColor)
		return nil
	}

	for _, step := range rollout {
		log.Info("canary step", "percent", step.Percent, "hold", step.HoldFor)
		if err := setCanaryPercent(ctx, canaryColor, step.Percent); err != nil {
			return err
		}

		if step.HoldFor > 0 {
			// Hold before checking, not after: metrics computed over a 5-minute
			// window need five minutes of traffic to mean anything.
			select {
			case <-time.After(step.HoldFor):
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		if err := checkGate(ctx, prom, canaryColor); err != nil {
			log.Error("gate failed; rolling canary back to 0%", "percent", step.Percent, "err", err)
			if rbErr := setCanaryPercent(ctx, canaryColor, 0); rbErr != nil {
				return errors.Join(err, fmt.Errorf("rollback also failed: %w", rbErr))
			}
			return err
		}
		log.Info("gate passed", "percent", step.Percent)
	}

	// Every rung held. Promote: the canary becomes the live color (the same
	// blue-green flip from Chapter 46), and the canary percentage goes back to
	// zero so the next deploy starts from a clean slate.
	if err := promote(ctx, canaryColor); err != nil {
		return err
	}
	log.Info("canary promoted to 100% and made live", "color", canaryColor)
	return nil
}

// checkGate returns nil if the canary is healthy, an error if it isn't.
func checkGate(ctx context.Context, prom promv1.API, canaryColor string) error {
	// Error rate: what fraction of the canary's responses are 5xx?
	//
	// The `or vector(0)` is not decoration. When the canary has served zero
	// 5xx responses, the numerator matches NO series, and in PromQL an empty
	// vector divided by anything is still empty — so a perfectly healthy
	// canary produces no result and a naive gate reads that as "cannot tell"
	// and aborts a good deploy. `or vector(0)` says the honest thing: no
	// errors is zero errors.
	q := fmt.Sprintf(`
		(sum(rate(http_requests_total{color="%s",status="5xx"}[5m])) or vector(0))
		/
		sum(rate(http_requests_total{color="%s"}[5m]))
	`, canaryColor, canaryColor)
	errRate, err := scalar(ctx, prom, q)
	if err != nil {
		return fmt.Errorf("query error rate: %w", err)
	}
	if errRate > 0.005 {
		return fmt.Errorf("canary error rate %.4f exceeds 0.5%%", errRate)
	}

	// p95 latency ratio against baseline — but only once there is enough
	// traffic for a p95 to mean anything.
	//
	// histogram_quantile is a estimate off bucket boundaries, so at low request
	// rates the p95 lands in a wide bucket and jumps around. Comparing two such
	// estimates doubles the noise, and a gate that fails on noise gets switched
	// off within a week, which is worse than not having one. minLatencyRPS is
	// the floor below which the comparison is skipped and said out loud.
	const minLatencyRPS = 5.0
	rps, err := scalar(ctx, prom, fmt.Sprintf(
		`sum(rate(http_requests_total{color=%q}[5m]))`, canaryColor))
	if err != nil {
		return fmt.Errorf("query request rate: %w", err)
	}
	if rps < minLatencyRPS {
		// Not a pass. An explicit "not measured", so nobody reads a green gate
		// as evidence the latency was checked.
		fmt.Fprintf(os.Stderr, "latency check SKIPPED: canary is serving %.2f req/s, below the %.0f req/s needed for a stable p95\n", rps, minLatencyRPS)
	} else {
		latRatio, err := scalar(ctx, prom, latencyRatioQuery(canaryColor))
		if err != nil {
			return fmt.Errorf("query latency: %w", err)
		}
		if latRatio > 1.25 {
			return fmt.Errorf("canary p95 latency %.2fx baseline", latRatio)
		}
	}

	// SLO budget. "Under the error threshold" and "not burning the month's
	// budget in an afternoon" are different questions, and this asks the second.
	burn, err := scalar(ctx, prom, sloBurnQuery(canaryColor))
	if err != nil {
		return fmt.Errorf("query slo burn: %w", err)
	}
	if burn > 2.0 {
		return fmt.Errorf("canary is burning error budget at %.2fx the sustainable rate", burn)
	}

	// new error fingerprints in the canary
	fps, err := newFingerprints(ctx, prom, canaryColor)
	if err != nil {
		return err
	}
	if len(fps) > 0 {
		return fmt.Errorf("canary surfaced %d new error fingerprints: %v", len(fps), fps)
	}
	return nil
}

func latencyRatioQuery(canaryColor string) string {
	stable := otherColor(canaryColor)
	return fmt.Sprintf(`
		histogram_quantile(0.95, sum by (le) (rate(http_request_duration_seconds_bucket{color="%s"}[5m])))
		/
		histogram_quantile(0.95, sum by (le) (rate(http_request_duration_seconds_bucket{color="%s"}[5m])))
	`, canaryColor, stable)
}

// sloBurnQuery expresses the canary's 5xx rate as a multiple of the rate that
// would exactly exhaust a 99.9% monthly availability budget. 1.0 means "on
// track to spend the entire month's budget in a month"; 2.0 means twice that.
func sloBurnQuery(canaryColor string) string {
	const budget = 0.001 // 99.9% availability
	return fmt.Sprintf(`
		(
		  (sum(rate(http_requests_total{color="%s",status="5xx"}[5m])) or vector(0))
		  /
		  sum(rate(http_requests_total{color="%s"}[5m]))
		) / %g
	`, canaryColor, canaryColor, budget)
}

// newFingerprints returns error signatures present in the canary and absent
// from the stable stack over the last day.
//
// This check fires even at a tiny absolute rate, on purpose. Three requests
// failing with an error nobody has ever seen before is not noise — it is the
// new deploy's bug, seen early, which is the entire reason for canarying.
func newFingerprints(ctx context.Context, prom promv1.API, canaryColor string) ([]string, error) {
	canary, err := labelValues(ctx, prom,
		fmt.Sprintf(`sum by (fingerprint) (increase(beacon_errors_total{color="%s"}[10m])) > 0`, canaryColor))
	if err != nil {
		return nil, fmt.Errorf("query canary fingerprints: %w", err)
	}
	stable, err := labelValues(ctx, prom,
		fmt.Sprintf(`sum by (fingerprint) (increase(beacon_errors_total{color="%s"}[1d])) > 0`, otherColor(canaryColor)))
	if err != nil {
		return nil, fmt.Errorf("query stable fingerprints: %w", err)
	}

	seen := make(map[string]bool, len(stable))
	for _, s := range stable {
		seen[s] = true
	}
	var fresh []string
	for _, c := range canary {
		if !seen[c] {
			fresh = append(fresh, c)
		}
	}
	return fresh, nil
}

func scalar(ctx context.Context, prom promv1.API, query string) (float64, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	val, warnings, err := prom.Query(ctx, strings.TrimSpace(query), time.Now())
	if err != nil {
		return 0, err
	}
	if len(warnings) > 0 {
		// Worth surfacing rather than swallowing: a warning here usually means
		// the query matched nothing, and a gate that silently passes because it
		// measured nothing is worse than no gate.
		return 0, fmt.Errorf("prometheus warnings: %v", warnings)
	}
	vec, ok := val.(model.Vector)
	if !ok || len(vec) == 0 {
		// No data is not zero. If the canary has served no traffic at all, the
		// honest answer is "cannot tell yet", and climbing on that basis is how
		// a broken deploy reaches 100%.
		return 0, fmt.Errorf("query returned no samples: %s", strings.TrimSpace(query))
	}
	f := float64(vec[0].Value)
	if f != f { // NaN, which a 0/0 division produces
		return 0, nil
	}
	return f, nil
}

func labelValues(ctx context.Context, prom promv1.API, query string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	val, _, err := prom.Query(ctx, strings.TrimSpace(query), time.Now())
	if err != nil {
		return nil, err
	}
	vec, ok := val.(model.Vector)
	if !ok {
		return nil, nil
	}
	out := make([]string, 0, len(vec))
	for _, s := range vec {
		out = append(out, string(s.Metric["fingerprint"]))
	}
	return out, nil
}

// setCanaryPercent writes the routing percentage the Cloudflare Worker reads.
func setCanaryPercent(ctx context.Context, color string, pct int) error {
	if err := kvPut(ctx, "canary_color", color); err != nil {
		return err
	}
	return kvPut(ctx, "canary_pct", fmt.Sprint(pct))
}

// promote makes the canary the live color and stops splitting traffic.
func promote(ctx context.Context, color string) error {
	if err := kvPut(ctx, "live_color", color); err != nil {
		return err
	}
	return kvPut(ctx, "canary_pct", "0")
}

func kvPut(ctx context.Context, key, value string) error {
	cmd := exec.CommandContext(ctx, "wrangler", "kv:key", "put", key, value, "--binding=BEACON_CONFIG")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("wrangler kv put %s=%s: %w: %s", key, value, err, out)
	}
	return nil
}

func otherColor(c string) string {
	if c == "green" {
		return "blue"
	}
	return "green"
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
