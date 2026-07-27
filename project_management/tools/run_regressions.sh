#!/bin/bash
# One-command regression runner for the spinifex Outposts-parity project
# (gate P0.2). Prints exactly one PASS/FAIL/SKIP line per suite and a
# final tally. Runtimes on this host are noted per suite so sessions can
# be budgeted. Run from anywhere; repos are located relative to this file.
#
# Suites:
#   spinifex-unit        go test, all packages EXCEPT spinifex/daemon
#                        (~60 s)
#   spinifex-daemon      the daemon package inside a hermetic netns --
#                        its TLS test stalls on the host network for
#                        reasons still under triage (~40 s)
#   spinifex-integration make test-integration (~15 s)
#   viperblock-unit      go test ./... in the viperblock clone (~5 min;
#                        spins its own predastore fixture on random ports)
#   predastore-unit      go test ./... in the predastore clone (~5 min;
#                        SKIPPED automatically if a dev cluster is up --
#                        fixture ports could collide)
#   crashharness-smoke   build + 2 SIGKILL cycles (~30 s)
set -u
PROJECT_TOOLS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SPINIFEX_DIR="$(cd "$PROJECT_TOOLS_DIR/../.." && pwd)"
VIPERBLOCK_DIR="$(cd "$SPINIFEX_DIR/../viperblock" && pwd)"
PREDASTORE_DIR="$(cd "$SPINIFEX_DIR/../predastore" && pwd)"
export GOTOOLCHAIN=auto
export GOWORK=off

PASS_COUNT=0; FAIL_COUNT=0; SKIP_COUNT=0
declare -a SUMMARY_LINES

record() { # record <suite> <PASS|FAIL|SKIP> <seconds> [note]
  local suite=$1 verdict=$2 seconds=$3 note=${4:-}
  SUMMARY_LINES+=("$(printf '%-22s %-4s %4ss %s' "$suite" "$verdict" "$seconds" "$note")")
  case $verdict in
    PASS) PASS_COUNT=$((PASS_COUNT+1));;
    FAIL) FAIL_COUNT=$((FAIL_COUNT+1));;
    SKIP) SKIP_COUNT=$((SKIP_COUNT+1));;
  esac
  printf '%-22s %s\n' "$suite" "$verdict"
}

run_suite() { # run_suite <name> <timeout-s> <command...>
  local name=$1 budget=$2; shift 2
  local started=$SECONDS
  if timeout "$budget" "$@" > "/tmp/regress-$name.log" 2>&1; then
    record "$name" PASS $((SECONDS-started))
  else
    record "$name" FAIL $((SECONDS-started)) "log: /tmp/regress-$name.log"
  fi
}

echo "=== spinifex project regression runner: $(date) ==="

# spinifex-unit: everything except the daemon package.
started=$SECONDS
if (cd "$SPINIFEX_DIR" && go list ./spinifex/... | grep -v 'spinifex/daemon$' \
    | timeout 900 xargs go test -count=1) > /tmp/regress-spinifex-unit.log 2>&1; then
  record spinifex-unit PASS $((SECONDS-started))
else
  record spinifex-unit FAIL $((SECONDS-started)) "log: /tmp/regress-spinifex-unit.log"
fi

# spinifex-daemon: hermetic netns (see ENVIRONMENT.md traps).
started=$SECONDS
if (cd "$SPINIFEX_DIR" && unshare -r -n sh -c \
    'ip link set lo up; GOWORK=off GOTOOLCHAIN=auto go test -count=1 ./spinifex/daemon/') \
    > /tmp/regress-spinifex-daemon.log 2>&1; then
  record spinifex-daemon PASS $((SECONDS-started))
else
  record spinifex-daemon FAIL $((SECONDS-started)) "log: /tmp/regress-spinifex-daemon.log"
fi

run_suite spinifex-integration 600 make -C "$SPINIFEX_DIR" test-integration

run_suite viperblock-unit 900 env -C "$VIPERBLOCK_DIR" go test -count=1 ./...

# predastore-unit: fixture ports may collide with a running dev cluster.
if pgrep -x s3d > /dev/null 2>&1; then
  record predastore-unit SKIP 0 "dev cluster (s3d) is running"
else
  run_suite predastore-unit 900 env -C "$PREDASTORE_DIR" go test -count=1 ./...
fi

# crashharness-smoke: 2 SIGKILL cycles through the public API. The harness
# lives on the feat/crash-consistency-harness branch; use a dedicated
# worktree so this suite does not depend on whichever branch the main
# viperblock clone has checked out (bit 2026-07-27: instant FAIL when the
# clone sat on the vhost-user branch).
CRASHHARNESS_WORKTREE="$VIPERBLOCK_DIR-crashharness-worktree"
if [ ! -d "$CRASHHARNESS_WORKTREE" ]; then
  git -C "$VIPERBLOCK_DIR" worktree add "$CRASHHARNESS_WORKTREE" \
    feat/crash-consistency-harness > /dev/null 2>&1
fi
started=$SECONDS
if (cd "$CRASHHARNESS_WORKTREE" && go build -o /tmp/regress-crashharness ./tests/crashharness/cmd/crashharness \
    && rm -rf /tmp/regress-ch && timeout 300 /tmp/regress-crashharness run \
       --dir /tmp/regress-ch --cycles 2 --min-run 1s --max-run 2s \
    | grep -q "ALL CRASHHARNESS CHECKS PASS") > /tmp/regress-crashharness.log 2>&1; then
  record crashharness-smoke PASS $((SECONDS-started))
else
  record crashharness-smoke FAIL $((SECONDS-started)) "log: /tmp/regress-crashharness.log"
fi
rm -rf /tmp/regress-ch /tmp/regress-ch_verify_scratch

echo
echo "=== summary ==="
for line in "${SUMMARY_LINES[@]}"; do echo "$line"; done
TOTAL=$((PASS_COUNT+FAIL_COUNT+SKIP_COUNT))
echo "tally: $PASS_COUNT/$TOTAL pass, $FAIL_COUNT fail, $SKIP_COUNT skip"
test "$FAIL_COUNT" -eq 0
