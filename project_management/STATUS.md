# STATUS -- Spinifex Outposts-parity + EBS-grade storage (dashboard)

<!-- Overwrite-in-place. History lives in JOURNAL.md. -->

Last updated: 2026-07-28 14:40 (P1.6 slices 2+3a DONE: batched flush
+ fsync off the WAL append path; in-guest clat max progression
246 s -> 35.3 s -> 31.6 s -> 16.3 s, diag steady IOPS 329 -> 3440;
slice 4 named: pipeline chunk assembly with the flush)
Current phase: Phase -1 nearly done (6/9 [x], 1/9 [~]) + Phase 1 OPEN
(P1.1 [~], P1.4 [~], P1.6 [~])
Current slice: NEXT = P1.6 slice 4: pipeline the drain (consume
flushed batches into chunks immediately so blocked writers see
headroom during, not after, the flush phase); then final bound + rig.

## NEEDS HUMAN

Nothing blocking. Non-blocking queue:
1. REVIEW (standing): F0/F1 were closed by delegated judgment call
   (DECISIONS.md) -- Outposts parity = service-set first then thin
   outposts.* API; "predastore as EBS" = harden the whole
   Viperblock+Predastore stack. Object if wrong.
2. NVMe pair (2x 1.9 TB, idle isw_raid members): destructive use
   permitted? Needed by the realistic-performance phases.
3. Optional: throwaway real AWS account for golden captures (fork F4).
4. Root access only needed later for bridged VM networking (P-1.2 can
   start with user networking).
5. Standing, low priority: gitcnd/{viperblock,predastore} are plain
   repos, not GitHub forks, so they cannot open PRs against mulgadc/*.
   If upstreaming becomes the goal: delete them, click Fork on the
   mulgadc repos, re-add to token, re-push (identical history, 2 min).
RESOLVED 2026-07-28: F7 ACKED by the human (verbatim in DECISIONS.md
F7) -- vhost-user-blk is the VM data path. F2 was acked earlier the
same day. OPS-1 (token rotation) resolved as NO-OP per human direction
(fork-scoped token, revoked after the work, merge-back reviewed).

## Security audit (2026-07-27, read-only, forked chat)
Preliminary hardening review at
project_management/SECURITY_AUDIT_PRELIMINARY.md (no code changed);
remediation queue inside. Three findings have committed reproducers
(SP-2 IAM condition drop, VB-1 close-after-failed-drain data loss,
VB-4 secret-key logging). OPS-1 resolved as no-op (see above). VB-1
overlaps Phase 1 durability work and should be fixed on that branch.

## Branch map (fork F6)

- spinifex (pushed to fork gitcnd/spinifex):
  - project-management: working documents (this folder).
  - feat/ebs-volume-types-and-snapshot-api, feat/ebs-qos-enforcement,
    feat/outposts-service-parity: created, no commits yet (Phase 3/4).
- ../viperblock (pushed to gitcnd/viperblock):
  - feat/data-path-vhost-user-blk: F7 winner -- pure-Go vhost-user-blk
    backend, boots real QEMU, ~1.8x NBD in-guest. ACTIVE (production
    engine wiring).
  - feat/replicated-wal-durability: Phase 1 -- SyncOnFlush barrier +
    walrepl peer replication (both landed, suite green).
  - feat/crash-consistency-harness: P-1.3 harness + strace evidence +
    perfbench (P-1.5).
  - feat/data-path-native-nbd-server: P-1.9 candidate (a), measured.
  - fix/nbd-close-open-race: lifecycle mutex fix (stands on mechanism
    + 70 clean post-fix cycles; differential repro closed 0/40).
- ../predastore (pushed to gitcnd/predastore):
  - feat/shard-healer-and-read-repair: availability probe + P-1.6
    baseline + s3-tests rig + P-1.7 frozen baseline. ACTIVE for Phase 2.
  - feat/s3-api-surface-completion: created, no commits yet (Phase 2).

## What landed 2026-07-28 (8th drop): P1.6 slices 2+3a
- Batched flush (commit df1b759): WAL appends off Writes.mu in
  4096-block batches, SeqNum-exact removal (racing rewrites survive,
  gated), flushMu, MarkPendingIfSeqNum, snapshot path via Flush().
  Differential gate: old flush stalls concurrent writes 1:1 (634 ms ==
  664 ms flush, FAIL); batched 14 ms vs 2.29 s flush (PASS). Suite
  green 179.8 s.
- WAL syncer fsync moved outside WAL.mu (commit 05064d6) -- giant
  fsyncs no longer block every WAL append behind them.
- In-guest: clat max 16.3 s (was 31.6 s post-slice-1-only... full
  progression 246 -> 35.3 -> 31.6 -> 16.3 s), diag steady IOPS 3440,
  matrix 4206/2262 rw. Artifact: viperblock/results/2026-07-28_p16_
  slice2_slice3_flush_and_fsync.txt. Slice 4 named: drain pipelining
  (headroom must appear DURING the flush phase, not after).

## What landed 2026-07-28 (7th drop): P1.6 admission-latency fix
- viperblock branch fix/backpressure-write-admission-latency (cut from
  main, pushed, commits 5e958b3 + b2a20d8): blocked writers now wait
  for per-chunk headroom at the high watermark; the background
  uploader owns drains; fail-fast contracts preserved. Unit gate test
  with TRUE differential validation (old contract FAILS 4.65 s, new
  PASSES 1.17 s); full suite green 185.8 s.
- In-guest re-run (same rig, merge build): steady randwrite 6.9-7.0x
  (592->4083 d1, 324->2261 d16), reads 2.3-2.6x, clat max 246 s ->
  35.3 s, p99 65 us unchanged, integrity green, raw control unchanged.
  Artifact: viperblock/results/2026-07-28_p16_backpressure_admission_
  fix.txt.
- Residual 35 s stall root-caused by probe: flushLocked holds
  Writes.mu for the whole flush (flush 724.7 ms == concurrent write
  max 721.5 ms, 1:1). Slice 2 named in P1.6: bounded flush batches +
  WAL segment rotation.

## What landed 2026-07-28 (6th drop): F7 production wiring + evidence
- vhost-user-blk-serve now serves the REAL viperblock WAL engine
  (--vb-backend file|s3): production open sequence, ErrZeroBlock
  translation, drain+close on SIGTERM (verified live), volume persists
  across close/reopen. Adapter test green under -race. viperblock
  commits bee3b5b + c28dfbe, pushed.
- In-guest evidence (one boot, engine and raw devices side by side):
  integrity green; transport EXONERATED (p99 write 65 us, burst
  27.3-36.2k IOPS ~ raw ceiling 35.4k) but steady-state randwrite
  592/324 IOPS d1/d16 -- single writes stall up to 246 s behind the
  synchronous backpressure drain (P-1.5(i) drain-stall, now proven
  guest-visible). Artifact: viperblock vhostuser/results/2026-07-28_
  inguest_fio_real_engine_vs_raw_vhost.txt.
- Gate P1.6 added in the open (sustained-write stalls bounded); the
  nbdkit-vs-vhost s3 production comparison is deferred until P1.6
  lands (both legs would measure the same stall today).
- Also: raw vhost leg rose 28.5k -> 35.4k IOPS after demoting hot-path
  per-request Info logs to Debug (~24% logging tax was measured into
  the frozen P-1.9 numbers; ratios and the F7 decision stand).

## What landed 2026-07-28 (5th drop): resume + F7 decided
- IDE-crash handoff consumed: state re-verified (all branches pushed,
  local==fork byte-identical, trees clean, no orphans), regression
  runner 6/6 green in 6m23s (spinifex-unit 67s, daemon-hermetic 26s,
  integration 2s, viperblock 192s, predastore 82s, crashharness 14s).
- F7 MAJOR fork DECIDED on human ack; OPS-1 no-op per human direction;
  STATUS drift fixed; temp_resume_handoff.md retired (content here);
  stale /tmp/vb-main-worktree pruned.

## What landed 2026-07-28 (4th drop): F7 evidence + Phase 1 slices
- P-1.9 [x]: vhost-user-blk boots real QEMU 7.2 + Debian 13 guest to
  SSH; /dev/vdb 16 MiB round-trip byte-identical; in-guest fio, same
  raw backend: vhost 28.5k randwrite IOPS vs nbdkit 15.7k (~1.8x),
  reads ~1.35x. Bugs fixed en route: bootindex, SET_VRING_NUM parse.
- P1.4 [~]: SyncOnFlush durable barrier landed + strace-verified on the
  served path (24 fsyncs across 20 barriers where pre-fix code
  produced zero across 50 flushes).
- P1.1 [~]: walrepl synchronous peer replication wired into the flush
  barrier; replica-WAL-byte-identical test green; fail-closed barrier.
- P-1.7 [x]: s3-tests baseline frozen 123/621/94 (Phase 2 floor).

## Next action
1. P1.6 slice 4 (the F7 critical path continues): pipeline the drain
   -- consume flushed batches into chunk assembly/upload immediately
   (or equivalent incremental hand-off in WriteWALToChunk) so a
   writer blocked at the watermark sees per-chunk headroom DURING the
   flush phase instead of after it (evidence: 16.3 s residual clat
   max == flush-phase length; see 8th drop). Then re-run the vhost
   rig, set the final 100 ms-class bound with the human, and run the
   deferred s3-backed nbdkit-vs-vhost production pair.
2. F2 next slice: promotion/recovery from replica WALs, reconnect/
   resync, degraded-mode policy; then close the acked-but-unflushed
   memory window (14.2% measured) via replicate-on-WriteAt or
   WAL-on-ack.
3. Phase 2 (predastore) when opened: healer + read-repair + degraded/
   quorum writes + metadata failover (P-1.6: 1 node down = writes 0%,
   reads 100% but ~2320x slower); then the S3 surface gaps (CopyObject,
   DeleteObjects, AbortMultipartUpload over HTTP, versioning).
4. Backlog: P-1.2 single-node VM deploy (rig VM ready at
   ../vm-images/rig-node1.qcow2, key rig_ssh_key); volatile-cache FUSE
   loss-count rig (P1.4 remainder); P0.3 golden-response harness;
   drain-stall fix (Phase 1 target, from P-1.5: writes stall minutes
   under sustained load); s3-tests subset suite into the regression
   runner (P0.2 remainder); VB-1 security fix on the durability branch.

## Gate snapshot
Phase -1: 6/9 [x] (P-1.1, P-1.3, P-1.4, P-1.5, P-1.7, P-1.9) + 1/9 [~]
(P-1.6) + open: P-1.2, P-1.8 . P0: 0/4 ticked (P0.2 runner exists,
validated 6/6 today; s3-tests subset still missing) . P1: 3/6 [~]
(P1.1, P1.4, P1.6 slice 1 done) . P2: 0/5 . P3: 0/6 . P4: 0/6 .
P5: 0/4

## Standing reminders
- ALL go commands: GOTOOLCHAIN=auto. Baselines additionally GOWORK=off.
- GOFIPS140=v1.0.0 for ANY binary embedding viperblock (fipsboot panic
  otherwise).
- Unit suites: normal mode EXCEPT spinifex/daemon which needs hermetic
  netns (`unshare -r -n` + lo up) -- TLS-test stall, cause under triage.
- NEVER print or commit the .env token; source .env inline per push.
- Feature branches cut from main (220141b1), not from project-management.
- Shared, loaded machine: VMs <= 16 GiB, kill orphans (pgrep -x, never
  pkill -f with self-matching patterns), nothing destructive without
  explicit ask.
- One-command regression: project_management/tools/run_regressions.sh.
- Predastore dev cluster without root: see ENVIRONMENT.md traps
  (pd-shim + SSL_CERT_FILE recipe).
- No validation artifact, no claim.
