#!/bin/bash
# s3-tests fast-subset suite for the spinifex regression runner (gate
# P0.2's s3-tests leg). Runs the curated passing subset in
# predastore tools/s3tests/regression_subset.txt against a 3-node
# loopback predastore cluster.
#
# Exit codes: 0 pass, 1 fail, 3 SKIP (rig absent or a foreign dev
# cluster is running -- this script only tests against a cluster whose
# lifecycle it owns).
#
# Requirements (see predastore tools/s3tests/README.md): the ceph/s3-tests
# clone at $S3TESTS_DIR with the cleanup patch applied, a .venv with
# requirements + pytest-timeout, predastore-loopback.conf copied in, and
# the /tmp/pd-shim no-root recipe from ENVIRONMENT.md.
set -u
S3TESTS_DIR="${S3TESTS_DIR:-/home/devnull/Downloads/github/s3-tests}"
PREDASTORE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/../predastore"
SUBSET_FILE="$PREDASTORE_DIR/tools/s3tests/regression_subset.txt"

if [ ! -x "$S3TESTS_DIR/.venv/bin/python" ] || [ ! -f "$S3TESTS_DIR/predastore-loopback.conf" ] || [ ! -f "$SUBSET_FILE" ]; then
  echo "SKIP: s3-tests rig not installed (see predastore tools/s3tests/README.md)"
  exit 3
fi
if pgrep -x s3d > /dev/null 2>&1; then
  echo "SKIP: a predastore dev cluster is already running (not ours to use)"
  exit 3
fi

STARTED_CLUSTER=0
cleanup() {
  if [ "$STARTED_CLUSTER" = 1 ]; then
    (cd "$PREDASTORE_DIR" && PATH=/tmp/pd-shim:$PATH ./scripts/stop.sh > /dev/null 2>&1)
    # stop.sh initiates a graceful shutdown that completes asynchronously
    # (observed 2026-07-29: s3d lingered ~1 min after stop.sh returned).
    # Wait for it before the hard-kill backstop (exact name match only).
    for _ in $(seq 1 20); do
      pgrep -x s3d > /dev/null 2>&1 || break
      sleep 3
    done
    pkill -x s3d 2>/dev/null
  fi
}
trap cleanup EXIT

(cd "$PREDASTORE_DIR" && setsid env PATH=/tmp/pd-shim:$PATH \
  SSL_CERT_FILE=/tmp/predastore/server.pem \
  ./scripts/start.sh -w 3node-loopback > /tmp/regress-s3tests-cluster.log 2>&1 &)
STARTED_CLUSTER=1
for i in $(seq 1 30); do
  sleep 2
  if [ "$(pgrep -x s3d | wc -l)" -ge 3 ]; then break; fi
done
if [ "$(pgrep -x s3d | wc -l)" -lt 3 ]; then
  echo "FAIL: loopback cluster did not start (log: /tmp/regress-s3tests-cluster.log)"
  exit 1
fi
sleep 5

mapfile -t SUBSET < <(grep -v '^#' "$SUBSET_FILE" | grep -v '^$')
cd "$S3TESTS_DIR"
S3TEST_CONF=predastore-loopback.conf AWS_DEFAULT_REGION=ap-southeast-2 \
  .venv/bin/python -m pytest --timeout=60 -q "${SUBSET[@]}"
