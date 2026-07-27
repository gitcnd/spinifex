# JOURNAL -- Spinifex Outposts-parity + EBS-grade storage (append-only)

## 2026-07-27 03:12 -- PROJECT INSTANTIATION: RESEARCH, MACHINE AUDIT, PLAN
Plan: onboard the engineering_development_methodology docs, deeply research
the Spinifex/Viperblock/Predastore stack and the AWS Outposts + EBS feature
surfaces, audit this machine, and instantiate the working-document system
with a phased plan.

Done:
- Methodology onboarded: all 12 docs + templates read
  (symlinks/FieldX3D/engineering_development_methodology/).
- Spinifex repo explored (subagent, full report retained in chat): SigV4
  gateway -> NATS -> stateless daemons; EC2 registry ~110+ actions
  (spinifex/gateway/ec2.go); EBS actions implemented: Create/Delete/
  Describe/Attach/Detach/ModifyVolume, DescribeVolumeStatus,
  DescribeVolumesModifications, encryption-by-default trio, Create/Delete/
  Describe/CopySnapshot. Only gp3 accepted
  (spinifex/handlers/ec2/volume/service_impl.go). Attach path: viperblockd
  -> nbdkit -> NBD URI -> QMP blockdev-add -> virtio-blk-pci
  (spinifex/vm/volumes.go).
- Viperblock v1.13.0 explored (module cache): 4 KiB blocks, 4 MiB chunks,
  WAL fsync every 200 ms, chunk upload every 30 s
  (viperblock/viperblock.go); writes ACKED FROM MEMORY before WAL fsync;
  guest FLUSH forces WAL flush; NO cross-node replication -- durability
  beyond the local WAL is delegated to the S3 backend. Snapshots are COW
  metadata freezes (viperblock/snapshot.go). Encryption AES-256-GCM exists.
  Published module zip is MISSING the nbd/ plugin source (packaging gap).
- Predastore v1.13.0 explored (module cache): Reed-Solomon (2+1 / 3+2),
  Raft metadata, QUIC shard nodes, AES-256-GCM at rest, fsync-disciplined
  segment writers. Healer/rebalance DOCUMENTED BUT NOT IMPLEMENTED
  (docs/TODO.md). Missing S3 surface: versioning, lifecycle, CORS,
  CopyObject, DeleteObjects, AbortMultipartUpload not wired over HTTP,
  ListParts/ListMultipartUploads.
- Outposts research (aws.amazon.com/outposts/rack/features/, AWS docs):
  racks locally run EC2, EBS + snapshots, S3, ECS, EKS, RDS, ALB,
  ElastiCache*, EMR*, Route 53 Resolver*, IoT Greengrass; local gateway
  (BGP) + CoIP; capacity management. Spinifex exceeds Outposts on local
  IAM/STS/ECR/NLB/Bedrock; lags on RDS, ElastiCache, EMR, Route 53
  Resolver, CloudWatch, LGW/CoIP semantics, outposts.* APIs.
- EBS research (aws.amazon.com/ebs/features/, EBS user guide): parity
  targets = ack-durable replicated writes (io2 99.999% durability),
  volume types with enforced QoS (gp3/io2 256k IOPS), Elastic Volumes live
  modify, incremental snapshots + FSR, multi-attach (io2, 16 instances,
  I/O fencing), KMS encryption, EBS direct APIs.
- Machine audited by probing (see ENVIRONMENT.md): 72-thread Xeon, 188 GiB
  RAM (only ~37 GiB free at audit), nested KVM enabled, /dev/kvm usable,
  Go bootstraps to 1.26.5 via GOTOOLCHAIN=auto (gateway package built
  clean), sudo needs password, nbdkit/OVN absent on host, host distro
  unsupported by install scripts.
- Working documents instantiated: STATUS.md, JOURNAL.md, DECISIONS.md
  (forks F0-F5), PHASE_GATES.md (Phases -1..5), ENVIRONMENT.md.

Failed/learned:
- "Fix predastore to be a true EBS alternative" is ambiguous: Predastore
  is the S3 layer; Viperblock is the EBS layer. Filed as MAJOR fork F1,
  leaning "harden the whole stack"; needs human ack before Phase 1.
- The nbd/ directory referenced by Viperblock's Makefile is absent from
  the v1.13.0 module zip -- work on the NBD path requires the git clone,
  not the module cache.
- readme.md (lowercase) does not exist in this repo; the file is README.md.

Metrics: gateway-package build ~52 s cold (includes toolchain+module
downloads). Full `go build ./...` launched in background this session;
result to be recorded in the next entry.

Fork movement: F0 opened (OPEN), F1 opened (LEANING a), F2 opened (OPEN),
F3 opened (LEANING KVM guests), F4 opened (LEANING botocore+golden),
F5 opened (LEANING ceph/s3-tests).

Next: get human answers on F0/F1 and the NEEDS HUMAN resource items; then
Phase -1 starting with P-1.1 (baseline unit+integration on this machine).

## 2026-07-27 05:45 -- P-1.1 UNIT BASELINE TRIAGED GREEN (two-mode union)
Plan: run the spinifex unit suite and triage failures (gate P-1.1).
Done:
- Normal mode (`GOWORK=off GOTOOLCHAIN=auto make test`, ~66 s): 2 package
  failures -- spinifex/daemon (TestClusterManager_TLSServesHTTPS) and
  services/viperblockd (TestEBSConfigQueueGroup_DetachedOpenFails).
- Root cause (measured, not assumed): the machine's resolver has an
  emsvr.com search domain with a WILDCARD A record; every nonexistent
  hostname resolves to 91.103.1.84 running a live HTTPS server. Fake test
  hosts (https://s3.mock.local) therefore produce slow TLS failures
  instead of instant DNS errors -> retries -> NATS timeouts. Verified:
  getent hosts s3.mock.local -> 91.103.1.84 (s3.mock.local.emsvr.com).
- Fix/workaround: hermetic netns (`unshare -r -n` + `ip link set lo up`).
  Both failing packages PASS hermetically. Full hermetic run (~37 s): 96
  ok, 3 different failures (admin TestDiscoverLocalIPs, gpu vfio tests,
  utils TestExtractDiskImageFromFile) -- all three need real interfaces or
  full userns permissions and PASS in normal mode.
- Conclusion: every package passes in at least one mode; every failure has
  a written environment attribution. Unit leg of P-1.1 marked done inside
  the [~]; integration tier launched, result to be journaled.
Failed/learned: wildcard DNS is a project-wide trap (banked in
  ENVIRONMENT.md with the netns recipe).
Metrics: unit suite 66 s normal / 37 s hermetic, 72-thread host.
Fork movement: none.
Next: integration triage, then fio baseline (P-1.5).
ADDENDUM 05:50: `make test-integration` ok (tests/integration 12.178 s,
zero failures) -> P-1.1 ticked [x].

## 2026-07-27 05:40 -- P-1.3 SIGKILL LEG COMPLETE; HARNESS FALSE-ACCUSATION
##                     INVESTIGATED AND FIXED
Plan: build the crash-consistency harness (gate P-1.3) on viperblock
branch feat/crash-consistency-harness and get an honest first number.
Done:
- Harness: tests/crashharness/ in the viperblock clone. Component-test
  discipline: drives viperblock ONLY through its public API (WriteAt /
  Flush / the production NBD-plugin open sequence: New -> Backend.Init ->
  LoadState -> EnsureVolumeUUID -> LoadLiveCheckpoint -> RecoverLocalWALs
  -> OpenWAL x2). Self-describing 4 KiB block patterns (magic + block +
  generation + CRC32), an acknowledged-write log with SESSION/W/FLUSH
  records, a controller that SIGKILLs the writer at random points, and a
  verifier that reopens through recovery and classifies every logged
  block. Verification runs on a COPY of the state so it cannot disturb
  the volume under test.
- RESULT (25 cycles, commit 40b9ef8, report tests/crashharness/results/
  2026-07-27_sigkill_25cycle_report.txt): 249635 acked writes, 136 flush
  barriers, 0 acknowledged-and-FLUSHed writes lost, 0 corrupt, 0 phantom;
  35401 acked-but-unflushed writes lost (14.2% of acked) = the measured
  memory-ack window. Viperblock's WAL + recovery pipeline is SOLID against
  process kill.
Failed/learned (the important part):
- The first harness version reported 100s of "lost flushed writes" per
  cycle. Full investigation (enumerated causes, copy-verification
  experiment, pause-writes recovery-only experiment, no-replay probe with
  LookupBlockToObject, backend md5 diff -- blocks.live.bin identical
  across snapshots, ack-log line-position analysis): the accusation was a
  HARNESS accounting bug. A FLUSH barrier only covers writes acknowledged
  by the SAME process lifetime; the cumulative log parser let a LATER
  session's FLUSH retroactively "cover" writes that had died unflushed in
  a killed process's memory. All 7584 suspect blocks were single-write
  gen-1 blocks acked after their session's last barrier. Fixed with
  SESSION markers + session-scoped promotion (documented in the parser).
  Lesson: when a brand-new test accuses mature code, suspect the test's
  semantics first -- and prove it either way with discriminating
  experiments, never by staring.
- vb.Flush() (the NBD guest-fsync barrier) writes WAL records to the file
  descriptor but does NOT fsync; only the 200 ms background syncer does
  (viperblock.go WriteWAL / flushLocked / StartWALSyncer). Guest fsync is
  therefore page-cache-durable (process kill: fine, proven above) but
  likely NOT power-loss durable. This is the headline Phase 1 fix target;
  the power-loss leg of P-1.3 (VM/QMP power cut) remains open to measure
  it.
- Viperblock's ReadAt returns ErrZeroBlock for never-persisted blocks
  (not zero-filled data) -- harness had to treat it as semantic zero.
- The file backend's Init requires its BaseDir to pre-exist.
Metrics: 25-cycle study ~8 min wall (verify cost grows with volume fill);
  writer sustains ~3.3k acked writes/s at 4 workers with 1 ms throttle.
Fork movement: none (F2 evidence pending P-1.4 prototype).
Next: P-1.4 WAL-replication latency prototype; power-loss leg of P-1.3.

## 2026-07-27 05:10 -- AUTONOMY WIRING: DELEGATED FORKS, BRANCHES, PUSHES
Plan: harden the plan for autonomous execution per the human's go-ahead.
Done:
- F0 and F1 closed by delegated judgment call (human: "check that the
  plan is strong enough to let you work on this with maximum autonomy,
  and lets go!"); both flagged for review in STATUS NEEDS HUMAN.
- F6 decided: branch-per-component workflow; spinifex pushes to fork
  gitcnd/spinifex (token verified push+admin); viperblock/predastore
  cloned as siblings at v1.13.0 (== module versions, no divergence) with
  local-only branches (token cannot fork: "Bad credentials" outside its
  scope). go.work (untracked) wires the workspace; clone-deps.sh's
  go.mod-replace approach rejected to keep the fork clean.
- Branches created and pushed to gitcnd/spinifex: project-management
  (working docs), feat/ebs-volume-types-and-snapshot-api,
  feat/ebs-qos-enforcement, feat/outposts-service-parity. Local branches:
  viperblock feat/crash-consistency-harness + feat/replicated-wal-
  durability; predastore feat/shard-healer-and-read-repair +
  feat/s3-api-surface-completion.
Failed/learned: exported env vars do not persist between agent shell
  calls (first push failed); .env must be sourced inline per push. Banked.
Metrics: none.
Fork movement: F0 OPEN->DECIDED (delegated); F1 LEANING->DECIDED
  (delegated); F6 created DECIDED.
Next: P-1.1 baseline; crash harness.

## 2026-07-27 03:25 -- addendum: full build green
Plan: record the background full-build result.
Done:
- `GOTOOLCHAIN=auto go build ./...` at commit 220141b1: exit 0 (~90 s cold
  including module downloads). /proc/mdstat confirmed no assembled arrays
  (the NVMe isw_raid_member drives are idle).
Failed/learned: none.
Metrics: full build ~90 s cold on 72-thread host.
Fork movement: none.
Next: as above (P-1.1 unit/integration baseline).
