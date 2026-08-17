// Chapter 45 — backups and disaster recovery.
//
// Everything here is built backwards from two numbers:
//
//	RPO  how much data you are willing to lose      Beacon targets 5 minutes
//	RTO  how long you are willing to be down        Beacon targets 1 hour
//
// Three layers get you there, and each covers a failure the others don't:
//
//  1. Provider snapshots — fast, and in the same region as the thing that just
//     died. Fine for "someone dropped a table", useless if the provider is the
//     problem.
//  2. This job: an off-provider pg_dump into different cloud storage. Slow, and
//     the only layer that survives your provider going away or your account
//     being closed.
//  3. WAL archiving — what actually makes a 5-minute RPO real. Daily dumps
//     alone cap you at a 24-hour RPO no matter how clever the rest is; you
//     replay the write-ahead log forward from a base backup to any timestamp
//     you like.
//
// [verbatim ch45's BackupWorker] adapted to this repo's queue: the chapter's
// River worker (river.WorkerDefaults + a Kind() method) becomes this queue's
// handler signature. The pipeline, the encryption and the S3 upload are
// unchanged.

package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// KindBackup is the job kind for an off-provider dump.
const KindBackup = "backup"

// BackupConfig is what the handler needs. [glue: the chapter reads these off
// its config.Config; internal/jobs must not import internal/config.]
type BackupConfig struct {
	SourceURL    string // the database to dump
	Bucket       string // the OFF-PROVIDER bucket — the whole point
	AgeRecipient string // age public key; the private key lives only in the safe
}

// BackupHandler runs pg_dump, compresses, encrypts, and streams the result
// straight into object storage.
//
// Read the pipeline carefully, because the reason for it is the interesting
// part: pg_dump | gzip | age is chained so the plaintext database NEVER touches
// the local disk. A backup file sitting unencrypted in /tmp on a machine you
// are about to throw away is a data breach with extra steps.
func BackupHandler(cfg BackupConfig, client *s3.Client, log *slog.Logger) Handler {
	return func(ctx context.Context, _ json.RawMessage) error {
		if cfg.Bucket == "" || cfg.AgeRecipient == "" {
			return fmt.Errorf("backup: BACKUP_BUCKET and BACKUP_AGE_RECIPIENT must be set")
		}

		stamp := time.Now().UTC().Format("2006-01-02T15-04-05Z")
		key := fmt.Sprintf("beacon/%s.sql.gz.age", stamp)

		// pg_dump | gzip | age -r <recipient>, then straight into S3.
		//
		// age is a small modern encryption tool: public-key encryption with one
		// short command. The recipient public key lives in the environment; the
		// private key lives only in the safe, which is what makes the bucket
		// safe to hold a copy of every customer's data.
		cmd := exec.CommandContext(ctx,
			"sh", "-c",
			fmt.Sprintf(
				"pg_dump --no-owner --no-acl --format=plain '%s' "+
					"| gzip -9 "+
					"| age -r '%s'",
				cfg.SourceURL,
				cfg.AgeRecipient,
			),
		)
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return fmt.Errorf("stdout pipe: %w", err)
		}
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("start pg_dump: %w", err)
		}

		_, err = client.PutObject(ctx, &s3.PutObjectInput{
			Bucket: &cfg.Bucket,
			Key:    &key,
			Body:   stdout,
			// Backups are written once and read approximately never — until the
			// day they are read exactly once, urgently.
			StorageClass: types.StorageClassStandardIa,
		})
		if err != nil {
			_ = cmd.Process.Kill()
			return fmt.Errorf("s3 put: %w", err)
		}
		if err := cmd.Wait(); err != nil {
			return fmt.Errorf("pg_dump wait: %w", err)
		}

		log.Info("backup complete", "key", key, "bucket", cfg.Bucket)
		return nil
	}
}
