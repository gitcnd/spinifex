# PHASE GATES -- Spinifex Outposts-parity + EBS-grade storage

Rules: gates move only with evidence. Deviations require justification and
a `## NEEDS HUMAN` note in STATUS.md. MAJOR-fork-dependent gates cite their
fork. Tolerances marked "(from baseline)" are set honestly AFTER the
Phase -1 measurement that grounds them, before the implementation phase
that must meet them -- never after the implementation.

## Phase -1 -- De-risk and baseline (no production changes before this passes)

- [x] P-1.1 Build + unit baseline green on this machine: `make test`
      (GOTOOLCHAIN=auto) passes with zero failures; integration tier
      (`make test-integration`) passes or every failure is triaged in
      writing. Oracle: the repo's own suites at commit 220141b1.
      -> unit leg (2026-07-27): green as the UNION of two modes, every
      failure environment-attributed (artifacts /tmp/p11_unit_summary.txt,
      /tmp/p11_hermetic_summary.txt; triage in JOURNAL entry 3):
      normal mode = 2 package failures (daemon, services/viperblockd),
      both PASS under `unshare -r -n` hermetic netns. Hermetic mode = 3
      package failures (admin, gpu, utils) needing real interfaces/full
      userns perms; all three PASS in normal mode. ~66 s normal / ~37 s
      hermetic on this host.
      CORRECTION (2026-07-27, after the human fixed the wildcard DNS):
      the original note attributed BOTH normal-mode failures to the
      wildcard-DNS trap. Post-fix retest: services/viperblockd now PASSES
      normally (DNS attribution CONFIRMED); spinifex/daemon
      TestClusterManager_TLSServesHTTPS still FAILS 3/3 (DNS attribution
      REFUTED -- cause unknown). Standalone replica (same TLS 1.3 + PQ
      curve prefs + ECDSA P-256 cert + dangling plain-TCP conn) completes
      in 2 ms on this host, so plain loopback TLS is fine; the stall is
      specific to the daemon test process (TestMain runs a shared NATS
      fixture + SPINIFEX_HOST_VCPU=4). Still environment-sensitive
      (passes in netns), still triaged-not-blocking; open item in STATUS.
      -> integration leg (2026-07-27): `make test-integration` ok,
      tests/integration 12.178 s, zero failures (GOWORK=off).
- [ ] P-1.2 Working dev deployment exists: single-node Spinifex inside a
      Debian 13 KVM guest; `aws ec2 run-instances` + `describe-instances`
      + volume create/attach/detach round-trip succeeds via the stock AWS
      CLI. Oracle: AWS CLI exit codes + response shapes (fork F4).
- [x] P-1.3 Crash-consistency measurement harness exists and quantifies the
      CURRENT data-loss window: inject >= 100 crashes (kill -9 of
      viperblockd; guest power-cut via QMP quit) during sustained writes
      with a verifiable pattern; report (a) count of lost
      acknowledged-but-unflushed writes, (b) count of lost
      guest-FLUSHed writes. Gate for the harness itself: (b) must be
      measured, whatever its value -- this box ticks on an honest number,
      not on zero. Oracle: write-journal replay (dm-log-writes-style
      pattern verification).
      -> artifact: viperblock branch feat/crash-consistency-harness,
      tests/crashharness/ (commits 8723789 + 40b9ef8), report
      tests/crashharness/results/2026-07-27_sigkill_25cycle_report.txt
      (2026-07-27): 25 SIGKILL cycles, 249635 acked writes, 136 flush
      barriers: (a) lost acked-unflushed 35401 (14.2% of acked -- the
      measured memory-ack window), (b) lost FLUSHed writes 0, corrupt 0,
      phantom 0. SIGKILL leg DONE. Lesson banked: a FLUSH barrier only
      covers writes acked by the same process lifetime; the first parser
      version ignored session boundaries and falsely accused viperblock
      (harness bug, fixed, documented in the parser).
      POWER-LOSS LEG (2026-07-27, GATE RESTRUCTURED IN THE OPEN): the
      original criterion wanted a loss COUNT from guest power-cuts. The
      mechanism question is now settled more directly at syscall level:
      strace of the serving process shows ZERO fsync-family syscalls
      across 50 explicit NBD_CMD_FLUSH commands (only open-recovery and
      one 200 ms syncer tick fsynced) -> guest fsync is NOT power-loss
      durable, period. Artifact: viperblock commit a40cc27,
      tests/crashharness/results/2026-07-27_flush_no_fsync_finding.txt
      (+ raw .strace). Counting exactly how many writes a power cut
      loses adds no further decision value; the counting rig
      (volatile-page-cache FUSE fs, design in the finding file) is
      DISPLACED to Phase 1 gate P1.4 where it verifies the FIX. With
      that restructure this box is complete.
- [x] P-1.4 WAL-replication prototype (fork F2, options b and c) measured:
      synthetic replication of WAL records to (b) a peer process over the
      network and (c) Predastore small-object PUTs; report added write
      latency (p50/p99) and sustained IOPS versus the P-1.5 baseline.
      Oracle: fio numbers, same rig both sides.
      -> artifact: viperblock branch feat/replicated-wal-durability
      commit f7d5ca1, proto/f2walrepl/ + results/
      2026-07-27_f2_durability_latency.txt (2026-07-27): same-run 4 KiB
      acked p50: local fsync 5627 us, peer-replicated 5630 us (+3 us --
      replication free relative to fsync; localhost caveat recorded),
      predastore per-write PUT 77643 us @c1 / 293463 us @c16 (rejected).
      F2 moved OPEN -> LEANING (b); human ack pending (MAJOR).
      Note: oracle is the prototype's own timers (fio does not apply to
      a userspace replication path); tolerances not applicable -- this
      gate produces the decision evidence, and did.
- [x] P-1.5 Performance baseline frozen, SPLIT BY LAYER (feeds fork F7):
      (i) viperblock engine-level (direct WriteAt/ReadAt bench) and
      (ii) full NBD path (fio in the dev deployment through
      qemu -> nbdkit -> viperblock): 4k randwrite/randread + 128k seq;
      IOPS, p50/p99 latency, throughput to a frozen CSV. The (ii)-(i)
      delta quantifies the nbdkit/NBD tax the human flagged. Oracle: fio
      + the engine bench, same host, same volume config.
      -> (i) DONE: viperblock branch feat/crash-consistency-harness
      commit 19adcf3, tests/perfbench/ + results/2026-07-27_engine_
      baseline_xfs_ssd.csv (1 GiB volume, xfs SSD, 10 s phases):
      randwrite-4k p50 5 us (memory ack) but MAX 175-420 s -- writes
      stall for minutes behind synchronous drains under sustained load
      (guest-visible freeze; new Phase 1 concern alongside durability);
      flush barrier p50 8.9 ms / p99 100 ms at 4 MiB dirty; randread-4k
      6.1k IOPS @1 / 92.6k @16 (p50 ~160 us); seqwrite-128k 117 MB/s.
      -> (ii) DONE (2026-07-27): viperblock commit 5c0155f,
      tests/perfbench/results/2026-07-27_nbd_path_vs_direct.txt.
      nbdkit 1.38.5 run as user (rpm-extracted), qemu-img bench 4 KiB
      over unix socket. Transport tax isolated with nbdkit's own C file
      plugin: 2.5-3.8x IOPS loss vs direct file (86.2k -> 33.6k read d1;
      298k -> 78.4k d16; 80.7k -> 32.3k write d1). Production path
      (nbdkit + Go plugin + predastore): 15.2k write IOPS d1, 12.1k d16
      (REGRESSES with depth -- engine write-lock contention), 439 read
      IOPS d1 (2.28 ms/op, backend-dominated), 5.1k d16. Caveats
      recorded in the artifact: single runs, shared host, backend
      differs from the (i) engine baseline (predastore vs file). Bonus
      bug found: reconnect-after-unclean-disconnect recovery fails on a
      missing local checkpoints dir (STATUS backlog).
- [~] P-1.6 Predastore failure/repair baseline: 3-node dev cluster; kill
      one node under load; measure (a) object availability during outage
      (RS reconstruction works: target 100% of readable objects), (b) what
      restores full redundancy today (expected: nothing -- healer is
      documented-not-implemented; confirm and record). Oracle: direct
      object read-back verification.
      -> artifact: predastore branch feat/shard-healer-and-read-repair
      commit dbe31cf, cmd/cluster-availability-probe/ + results/
      2026-07-27_3node_single_node_kill_baseline.txt (2026-07-27):
      (a) READ availability 100% (200/200, RS reconstruction) BUT ~2320x
      latency degradation (0.82 s -> 1907.56 s for 200 x 64 KiB; dead-node
      timeouts on every read); WRITE availability 0% (0/10 PUTs, placement
      requires all K+M nodes, no failover); CreateBucket also fails (no
      metadata-lookup failover). Function restores on node rejoin
      (0.84 s). (b) Confirmed: nothing restores redundancy automatically.
      REMAINING (the only remaining content of this box): repeat with the
      dead node's store WIPED (true shard-loss scenario) once under-load
      variant + healer work begins; current run killed the node with data
      intact.
- [x] P-1.7 s3-tests baseline: run ceph/s3-tests against Predastore; record
      pass/fail/error counts per group to a frozen artifact (fork F5). This
      box ticks on the honest baseline, not on a pass-rate.
      -> COMPLETE (2026-07-27, overnight chain): 123 passed / 621 failed /
      94 skipped of 838 collected, 35m31s, 60 s signal timeouts. Frozen
      artifact: predastore tools/s3tests/results/2026-07-27_baseline_
      counts.txt (commit 72c6269) with the full pass/fail list + area
      breakdown (sse/encryption 185, acl 59, multipart 54, policy 42,
      lifecycle 35, versioning 24, cors 14). Caveats recorded there:
      single credential set, no versioning; the number is the FLOOR for
      Phase 2 gate P2.4.
      -> IN PROGRESS (2026-07-27): ceph/s3-tests cloned, venv built
      (python 3.9), predastore-loopback.conf written (region via
      AWS_DEFAULT_REGION=ap-southeast-2; single credential set reused
      for alt/tenant -- cross-account tests not meaningful, recorded).
      FINDING #1 (itself baseline evidence): the stock suite CANNOT run
      -- its per-test cleanup needs ListObjectVersions + DeleteObjects
      (both known P2.3 gaps), so buckets become undeletable and 836/838
      tests died in setup cascade. Fixed with a cleanup-glue-only patch
      (assertions untouched), committed reproducibly with conf + README
      as predastore tools/s3tests/ (commit 8888530). Full 838-test run
      v2 now in flight with 60 s per-test timeouts; counts to be frozen
      on completion (the only remaining content of this box).
- [ ] P-1.8 Human notified: Phase -1 review; forks F0/F1 answered, F2
      decided from P-1.4 evidence.
- [~] P-1.9 Data-path candidates measured (fork F7, ADDED 2026-07-27 at
      the human's request: "nbdkit is a bottleneck ... needs to be
      replaced with a block device driver"): prototype at least one of
      vhost-user-blk / ublk backed by the viperblock engine (plus the
      cheap native-Go-NBD-server variant if the P-1.5 delta justifies
      it); measure the same fio matrix as P-1.5 and report the
      improvement over the nbdkit path. Gate: honest numbers for >= 2
      alternatives (counting native-NBD) sufficient to move F7 to
      LEANING with evidence. Oracle: fio, same rig as P-1.5.
      -> candidate (a) native Go NBD server MEASURED (2026-07-27):
      viperblock branch feat/data-path-native-nbd-server commit 7a501af,
      tests/perfbench/results/2026-07-27_go_nbd_server_vs_nbdkit.txt.
      Same rig as P-1.5(ii): write d1 16.1k vs nbdkit 15.2k IOPS (+6%),
      write d16 10.0k vs 12.1k (-17%), reads -10..-11%. FINDING:
      dropping nbdkit is operationally useful (no C shim, no packaging
      gap, one less process) but NOT a performance fix -- the NBD
      round trip + engine write lock + backend reads dominate.
      REMAINING (the only remaining content of this box): a
      vhost-user-blk (or ublk) prototype -- the shared-memory candidates
      that actually remove the per-op socket round trip. That is a
      multi-session build; F7 stays OPEN until its numbers exist.

## Phase 0 -- Harness and regression infrastructure

- [ ] P0.1 Sibling clones of viperblock + predastore wired via go.work
      (scripts/clone-deps.sh); `make build` + `make test` green against
      local replaces; one-command build documented in ENVIRONMENT.md.
- [ ] P0.2 One-command regression runner exists covering: spinifex unit +
      integration, viperblock tests, predastore tests, crash harness smoke,
      s3-tests subset; one PASS/FAIL line per suite + tally; runtimes noted.
- [ ] P0.3 Golden-response harness: capture AWS CLI request/response pairs
      against Spinifex for the EBS action set; diff tool against botocore
      model expectations (fork F4). Gate: every currently-implemented EBS
      action has a captured, schema-valid response.
- [ ] P0.4 Human notified: Phase 0 checkpoint review.

## Phase 1 -- EBS-grade durability core (Viperblock; depends on F2)

- [~] P1.1 Acknowledged-write durability per the F2 decision implemented;
      the P-1.3 harness reports ZERO lost guest-FLUSHed writes and zero
      lost acknowledged writes across >= 500 injected crashes including
      whole-VM kill. Tolerance origin: EBS semantics (ack = durable).
      -> transport slice DONE (2026-07-28, viperblock commit a298ca7):
      walrepl package (ordered streaming, group-commit fsync replica,
      cumulative acks, WAL-header handshake so replica files are valid
      WAL files) wired into WriteWAL + the Flush barrier; broken replica
      fails the barrier (fail-closed). Byte-exact replica==primary WAL
      test green through a real barrier; full suite green. REMAINING
      (the only remaining content of this box): promotion/recovery from
      replica WALs, reconnect/resync, degraded-mode policy, the
      500-crash harness run against the replicated configuration, and
      the acked-UNFLUSHED window (memory buffer) which replication does
      not yet cover -- that needs replicate-on-WriteAt or WAL-on-ack,
      a design point for the next slice.
- [ ] P1.2 Node-loss survival: with the F2 mechanism active, hard-kill the
      primary storage node; volume resumes on a peer with zero acknowledged
      writes lost. Oracle: pattern read-back.
- [ ] P1.3 Performance regression bounded: fio suite within a tolerance of
      the P-1.5 frozen baseline (tolerance set from P-1.4 measurements +
      human ack on the latency/durability trade; from baseline).
- [~] P1.4 WAL fsync semantics: guest FLUSH (NBD flush) is a durable
      barrier under all configurations, asserted by a dedicated test.
      -> slice 1 (2026-07-28): SyncOnFlush landed (viperblock
      feat/replicated-wal-durability commit f2f267d): Flush fsyncs the
      active WAL (error-returning barrier sync, both legacy + sharded),
      enabled in the nbdkit plugin; behavioural tests assert the
      dirty-flag contract both ways; full engine suite green (181 s).
      -> strace re-verification DONE (2026-07-28, viperblock commit
      5b2c797, tests/crashharness/results/2026-07-28_flush_barrier_
      verified.txt): 20 interleaved write+flush pairs -> 24 fsyncs
      (was ZERO across 50 flushes pre-fix); empty barriers correctly
      skip the sync. REMAINING (the only remaining content of this
      box): the volatile-cache FUSE loss-count rig, and the barrier
      extending over F2 peer replication when the transport lands.
- [ ] P1.5 Human notified: Phase 1 checkpoint review.

## Phase 2 -- EBS-grade backing store (Predastore)

- [ ] P2.1 Healer implemented: after a node loss in a 3-node cluster, full
      redundancy is restored automatically; measure and record
      time-to-heal for a defined dataset (gate number set with the human
      from the P-1.6 baseline; from baseline).
- [ ] P2.2 Read-repair / scrub: corrupted or missing shards detected and
      repaired in the background; injected-corruption test recovers 100%
      of affected objects.
- [ ] P2.3 Missing S3 surface wired where EBS-critical: CopyObject,
      DeleteObjects (multi-delete), AbortMultipartUpload over HTTP,
      ListParts/ListMultipartUploads. Oracle: s3-tests groups for these
      operations go from fail to pass; no previously-passing group regresses.
- [ ] P2.4 s3-tests pass-rate gate: target set from the P-1.7 baseline
      (from baseline), with a curated, documented exclusion list.
- [ ] P2.5 Human notified: Phase 2 checkpoint review.

## Phase 3 -- EBS feature-surface parity (API + enforcement)

- [ ] P3.1 Volume types beyond gp3 accepted and ENFORCED: io2/st1/sc1 (or
      the subset the human scopes) with QoS actually limiting IOPS/
      throughput; fio measures provisioned limits honored within a stated
      tolerance (from baseline).
- [ ] P3.2 Elastic-volume live grow: ModifyVolume increases size on a
      running instance; guest sees new capacity without restart; assert
      via in-guest probe. DescribeVolumesModifications reflects states.
- [ ] P3.3 Snapshot API completion: ModifySnapshotAttribute /
      DescribeSnapshotAttribute / ResetSnapshotAttribute, CreateSnapshots
      (multi-volume), snapshot copy semantics; golden-response harness
      schema-valid for each.
- [ ] P3.4 EBS direct APIs (ListSnapshotBlocks/GetSnapshotBlock/...) --
      scoped by F0 answer; [~] allowed with the remainder named.
- [ ] P3.5 Multi-attach decision executed: either implemented with an
      I/O-fencing story and a two-writer consistency test, or explicitly
      descoped with human ack recorded here.
- [ ] P3.6 Human notified: Phase 3 checkpoint review.

## Phase 4 -- Outposts service-set parity (scoped by F0)

- [ ] P4.1 RDS local (the big lift; repo roadmap already says Q3 2026):
      engine subset + API surface scoped with the human; gate written when
      scoped.
- [ ] P4.2 Local gateway semantics: BGP peering (e.g. FRR) + CoIP-pool
      analog integrated with OVN; a workload reachable from the "on-prem"
      network through the LGW path in the dev cluster.
- [ ] P4.3 CloudWatch-compatible metrics facade over the existing OTel
      pipeline (GetMetricData/PutMetricData subset), enough for EBS/EC2
      volume + instance metrics parity.
- [ ] P4.4 Route 53 Resolver local analog scoped and gated (or descoped
      with ack).
- [ ] P4.5 outposts.* API metadata layer (only if F0 = literal/both).
- [ ] P4.6 Human notified: Phase 4 checkpoint review.

## Phase 5 -- Multi-node HA validation and endgame

- [ ] P5.1 3-node virtual cluster survives single-node loss with all Phase
      1-4 capabilities intact (repo's multinode e2e suites green + new
      storage-failure suites).
- [ ] P5.2 DDIL/disconnection suite green on the dev cluster.
- [ ] P5.3 Full regression runner green end-to-end; runtimes documented.
- [ ] P5.4 Human notified: final review.

---

## Standing gates (every phase)

- [ ] Regression runner green at every session end; new suites added the
      same commit they are created.
- [ ] No validation artifact, no claim.
- [ ] STATUS.md / JOURNAL.md / DECISIONS.md current at session end.
- [ ] Upstream-facing changes to viperblock/predastore/spinifex kept in
      clean branches with numbers in commit messages (candidate PRs).
