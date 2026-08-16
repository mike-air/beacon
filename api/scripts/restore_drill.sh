#!/usr/bin/env bash
# Chapter 45 — the restore drill.
#
# A backup nobody has restored is not a backup, it is a hope. This script is
# what turns the hope into a fact, and it is meant to run on a schedule, not
# only on the worst day of the quarter.
#
#   RESTORE_TARGET_URL=postgres://.../beacon_restore_drill ./scripts/restore_drill.sh
set -euo pipefail

: "${BACKUP_BUCKET:?set BACKUP_BUCKET}"
: "${RESTORE_TARGET_URL:?set RESTORE_TARGET_URL — a THROWAWAY database}"
: "${BACKUP_AGE_IDENTITY:=/secret/backup.key}"

# 1. Pick the most recent dump.
LATEST=$(aws s3 ls "s3://${BACKUP_BUCKET}/beacon/" | sort | tail -1 | awk '{print $4}')
echo "Restoring $LATEST"

# 2. Stream it back through age and gzip into a fresh test DB.
#    Note the direction: nothing is written to local disk in plaintext here
#    either, for the same reason it wasn't on the way out.
aws s3 cp "s3://${BACKUP_BUCKET}/beacon/$LATEST" - \
  | age --decrypt --identity "$BACKUP_AGE_IDENTITY" \
  | gunzip \
  | psql "$RESTORE_TARGET_URL"

# 3. Run the smoke queries.
psql "$RESTORE_TARGET_URL" -f scripts/restore_smoke.sql

# 4. If we got here, the drill passed.
echo "Restore drill PASSED — $(date -u)"
