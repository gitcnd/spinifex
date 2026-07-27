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
      both caused by the machine's wildcard-DNS trap (fake host
      s3.mock.local resolves to a real server; both packages PASS under
      `unshare -r -n` hermetic netns). Hermetic mode = 3 package failures
      (admin, gpu, utils) needing real interfaces/full userns perms; all
      three PASS in normal mode. ~66 s normal / ~37 s hermetic on this
      host.
      -> integration leg (2026-07-27): `make test-integration` ok,
      tests/integration 12.178 s, zero failures (GOWORK=off).
- [ ] P-1.2 Working dev deployment exists: single-node Spinifex inside a
      Debian 13 KVM guest; `aws ec2 run-instances` + `describe-instances`
      + volume create/attach/detach round-trip succeeds via the stock AWS
      CLI. Oracle: AWS CLI exit codes + response shapes (fork F4).
- [~] P-1.3 Crash-consistency measurement harness exists and quantifies the
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
      phantom 0. SIGKILL leg DONE. REMAINING (the only remaining content
      of this box): host power-loss leg -- code reading shows vb.Flush()
      writes WAL records without fsync (only the 200 ms background syncer
      fsyncs), so guest fsync is likely NOT power-loss durable; needs the
      VM/QMP power-cut harness to measure, and is the headline Phase 1 fix
      target either way. Lesson banked: a FLUSH barrier only covers writes
      acked by the same process lifetime; the first parser version ignored
      session boundaries and falsely accused viperblock (harness bug,
      fixed, documented in the parser).
- [ ] P-1.4 WAL-replication prototype (fork F2, options b and c) measured:
      synthetic replication of WAL records to (b) a peer process over the
      network and (c) Predastore small-object PUTs; report added write
      latency (p50/p99) and sustained IOPS versus the P-1.5 baseline.
      Oracle: fio numbers, same rig both sides.
- [ ] P-1.5 Performance baseline frozen: fio 4k randwrite/randread and
      128k seq through the NBD path on the dev deployment; IOPS, p50/p99
      latency, throughput recorded to a frozen CSV. Oracle: fio.
- [ ] P-1.6 Predastore failure/repair baseline: 3-node dev cluster; kill
      one node under load; measure (a) object availability during outage
      (RS reconstruction works: target 100% of readable objects), (b) what
      restores full redundancy today (expected: nothing -- healer is
      documented-not-implemented; confirm and record). Oracle: direct
      object read-back verification.
- [ ] P-1.7 s3-tests baseline: run ceph/s3-tests against Predastore; record
      pass/fail/error counts per group to a frozen artifact (fork F5). This
      box ticks on the honest baseline, not on a pass-rate.
- [ ] P-1.8 Human notified: Phase -1 review; forks F0/F1 answered, F2
      decided from P-1.4 evidence.

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

- [ ] P1.1 Acknowledged-write durability per the F2 decision implemented;
      the P-1.3 harness reports ZERO lost guest-FLUSHed writes and zero
      lost acknowledged writes across >= 500 injected crashes including
      whole-VM kill. Tolerance origin: EBS semantics (ack = durable).
- [ ] P1.2 Node-loss survival: with the F2 mechanism active, hard-kill the
      primary storage node; volume resumes on a peer with zero acknowledged
      writes lost. Oracle: pattern read-back.
- [ ] P1.3 Performance regression bounded: fio suite within a tolerance of
      the P-1.5 frozen baseline (tolerance set from P-1.4 measurements +
      human ack on the latency/durability trade; from baseline).
- [ ] P1.4 WAL fsync semantics: guest FLUSH (NBD flush) is a durable
      barrier under all configurations, asserted by a dedicated test.
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
