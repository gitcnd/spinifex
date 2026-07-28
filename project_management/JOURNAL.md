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

## 2026-07-27 14:30 -- SECURITY AUDIT: RUNTIME CONFIRMATION OF 3 FINDINGS
Plan: (forked chat) deeper runtime checks on the preliminary audit
findings WITHOUT disturbing the running s3-tests / shared cluster.
Status check first: s3-tests run had already ENDED (no live pytest
proc; exit 1) but NOT cleanly -- it hung in an SSL read on one test and
produced no pass/fail summary, so P-1.7 still needs a clean rerun (other
thread's task). The completion watcher I armed earlier is BROKEN: its
own command line matches its `pgrep -f "pytest s3tests/functional"`
pattern, so it self-matches and will never fire (the other thread's
await will hang). Flagged to the human; did not kill it (other thread
depends on it). 3-node predastore cluster still up; left untouched.
Done (isolated reproducers, file-backend/in-memory, no ports, no
cluster -- project_management/security_repros/, go.mod replace -> local
clones):
- SP-2 CONFIRMED: an Allow gated by Condition{MFA} unmarshals with the
  Condition dropped; Evaluate returns Allow (1) with no MFA context.
  Over-grant is real, not just theoretical.
- VB-1 CONFIRMED (CRITICAL, data loss): backend wrapper fails only
  FileTypeChunk writes; WriteAt+Flush a block (present pre-close), Close
  fails the chunk upload but SaveState/SaveBlockState succeed so
  RemoveLocalFiles runs (0 WAL files left), reopen reads ZERO BLOCK.
  A flushed, acknowledged write was permanently lost.
- VB-4 CONFIRMED: the nbd plugin's slog call renders SecretKey AND
  AccessKey verbatim into the JSON log (also under !BADKEY, VolumeSize:0
  -- a LogValue() fixes both).
- Updated SECURITY_AUDIT_PRELIMINARY.md with RUNTIME lines + status;
  committed the reproducers with a README (replace-path caveat noted).
Failed/learned:
- pgrep watcher patterns must not appear in the watcher's own command
  line (self-match). Same family as the earlier pkill-self trap. Use a
  sentinel file or match the python binary, not the pytest arg string.
Metrics: three go-run reproducers ~10s each after first compile.
Fork movement: none (audit only).
Next (remediation agent, not this chat): reproducers exist for SP-2 /
VB-1 / VB-4 -- fix + re-run to green; build reproducers for the rest.

## 2026-07-27 13:55 -- PRELIMINARY SECURITY HARDENING REVIEW (READ-ONLY)
Plan: (forked chat, this branch only) human asked for a read-only
preliminary security audit across the three repos, documented as a
handoff for a later remediation agent -- change nothing. Explicitly
avoided the running cluster/tests (the other forked chat is using them).
Done:
- Three parallel read-only reviews (spinifex, viperblock, predastore)
  plus manual spot-checks; consolidated into
  project_management/SECURITY_AUDIT_PRELIMINARY.md with stable IDs
  (SP-/VB-/PD-/OPS-), severities, evidence citations, and a
  confirm-then-fix priority queue. NO code changed.
- Headline provisional findings (static reading, NOT yet reproduced):
  VB-1 Close-after-failed-drain can delete unrecovered WAL (critical,
  data loss); PD-1/PD-2 QUIC + Raft transports lack client-cert auth
  (critical); SP-4 authorization fail-open branches; SP-1 STS not gated
  by caller identity policy; SP-2/SP-3 IAM Condition blocks silently
  dropped (over-grant, incl. sts:ExternalId); PD-3/PD-4 streaming
  per-chunk sig + trailer checksum not verified; several
  limit-before-allocate gaps (SP-6, VB-3, PD-5, PD-6); secrets on argv /
  in logs (SP-5, VB-4).
- Recorded the confirmed-SAFE list per repo too, so the remediation
  agent does not chase false positives (constant-time SigV4 compare,
  default-deny IAM, GCM nonce discipline, TLS 1.3 floors, content-hash
  storage keys, etc.).
Failed/learned:
- Framed the whole review in neutral defensive language after the first
  run's truncated prompt; kept it strictly read-only and off the shared
  cluster.
Metrics: three subagent reviews ~ parallel, ~15 min wall.
Fork movement: none (audit only; no forks touched).
Next (for whoever picks up remediation, NOT this chat): work the
SECURITY_AUDIT_PRELIMINARY.md priority queue, reproduce each item before
fixing. OPS-1 (rotate the working-tree token) is a human action.

## 2026-07-28 10:15 -- RESUMED AFTER IDE CRASH (NEW AGENT); F7 ACKED ->
##                     DECIDED; DOCS REFRESHED
Plan: (new agent) consume temp_resume_handoff.md, verify ground state,
confirm green baseline, record the human's F7 ack, fix doc drift.
Done:
- State verification: all three repos exactly at the handoff's HEADs
  (spinifex project-management 012fcd72, viperblock a2fc287, predastore
  72c6269), trees clean, every local branch byte-identical to its
  gitcnd fork ref, no orphaned processes (exact-name pgrep sweep).
- Regression runner 6/6 PASS in 6m23s (spinifex-unit 67s,
  daemon-hermetic 26s, integration 2s, viperblock-unit 192s,
  predastore-unit 82s, crashharness-smoke 14s). Baseline green.
- F7 DECIDED: human ack recorded verbatim in DECISIONS.md ("It is a
  "1b ack" from me ... and also an F7-ack ... please get going!").
  vhost-user-blk is the VM data path; production wiring starts now.
- OPS-1 resolved as NO-OP per human direction (verbatim in the audit
  file): fork-scoped token, revoked after the work, merge-back
  reviewed. Remediation queue updated.
- Doc drift fixed: STATUS rewritten (stale header lines, gate snapshot
  1/8 -> 6/9 [x], next-action queue refreshed from the handoff);
  temp_resume_handoff.md retired (consumed; authoritative state lives
  in STATUS again); stale /tmp/vb-main-worktree pruned from viperblock.
Failed/learned:
- The ENVIRONMENT.md pgrep-self-match trap bit the FIRST verification
  command of the resume (pgrep -f matched the invoking shell's command
  line). Cost seconds instead of an investigation because the trap was
  banked -- the traps file earns its keep on day one of a new agent.
Metrics: full re-orientation (work-log + 13 methodology docs + working
docs + state verification + 6/6 runner) ~35 min wall.
Fork movement: F7 LEANING (b) -> DECIDED (human ack).
Next: wire the real viperblock WAL engine behind the vhost-user backend
(serve mode), boot the rig guest against it, re-run in-guest fio
(STATUS next-action 1).

## 2026-07-28 09:10 -- F7 DECIDED BY EVIDENCE: VHOST-USER-BLK BOOTS REAL
##                     QEMU AND IS ~1.8x NBD IN-GUEST (P-1.9 [x])
Plan: chase the "next path" -- get the vhost-user backend booting a real
guest and produce the F7 decision numbers.
Done (this was a big one):
- Root-caused BOTH prior blockers, neither was what I'd guessed:
  1. The "QEMU 96% spin / empty serial" was BOOT ORDER -- SeaBIOS tried
     to boot the blank vhost disk and hung. Proven via a reference
     oracle (qemu-storage-daemon's own vhost-user-blk export showed the
     SAME empty serial) and an isolation boot (memfd machine boots fine
     with NO vhost device). Fix: bootindex=0 on the OS disk.
  2. Then the guest hung in initramfs (udev timeout). Backend logging
     against the live guest showed size=0 at SET_VRING_KICK -> queue
     never started. Bug: SET_VRING_NUM/BASE payload is
     struct{u32 index; u32 num}; I read a bare u64's low bits (=index=0)
     instead of num at offset 4. Fixed (commit e0423c7).
- RESULT: guest boots to SSH with our pure-Go vhost-user-blk backend;
  /dev/vdb round-trips 16 MiB random data byte-identical. Then the F7
  decision measurement (commit a2fc287, in-guest fio, same raw backend,
  three paths in one guest):
    vhost-user 28.5k/28.3k rw d1/d16, 24.8k/25.5k rd
    nbdkit     15.7k/16.4k rw,        18.4k/18.3k rd
    plain vio  14.3k/16.1k rw,        19.0k/18.2k rd
  vhost-user ~1.8x NBD on writes, ~1.35x reads, and beats QEMU's own
  virtio-blk-over-file. P-1.9 [x]; F7 -> LEANING (b) (human ack pending,
  MAJOR).
Failed/learned:
- Don't trust one success signal: serial emptiness was a machine-config
  artifact, not the bug. The reference oracle corrected me.
- The shell harness's silent && breakage on non-zero pgrep/grep + the
  pidfile-subshell trap cost real time again; I moved to editor-based
  instrumentation (reliable) over python/sed heredocs and to explicit
  process loops. Enough scalps now that this is a standing rule.
Metrics: this arc ~2.5 h incl. ~6 guest boots (~2-4 min each).
Fork movement: F7 OPEN -> LEANING (b) with decisive evidence.
Next: human ack on F7; then wire the real viperblock WAL engine behind
vhost-user (+ the engine write-concurrency qualifier) and multi-queue /
reconnect; the raw-file result isolates transport, which was the
question.

## 2026-07-28 06:30 -- MEM-TABLE REMAP FIX LANDED; QEMU WEDGE RE-DIAGNOSED
##                     WITH A REFERENCE-BACKEND ORACLE (SPLIT IN TWO)
Plan: fix the mem-table-remap-after-start wedge from the last smoke test.
Done:
- Remap fix implemented + committed (viperblock commit 0227dd0,
  feat/data-path-vhost-user-blk): a queueAccessMutex serialises the
  service goroutine's ring access against SET_MEM_TABLE; on a mem-table
  change while running, old regions are munmapped and the live vring is
  rebuilt from its stored user-addresses against the new mapping,
  preserving the avail cursor. In-process functional test EXTENDED to
  re-send SET_MEM_TABLE mid-stream then write/read-verify; green under
  -race. This is correct and needed regardless of the QEMU issue.
- BUT the remap fix did NOT clear the real-QEMU wedge -> re-diagnosed
  properly with a REFERENCE ORACLE (methodology: independent trusted
  tool). Ran the IDENTICAL qemu command against qemu-storage-daemon's
  own vhost-user-blk export. Findings that split the problem cleanly:
  (1) Empty guest serial happens with the REFERENCE backend too (0
      bytes) -> the empty-serial signal is a MACHINE-CONFIG artifact
      (q35 + memory-backend-memfd + this cloud image's console
      routing), NOT my backend. My earlier "wedge" read over-relied on
      serial as the success signal.
  (2) CPU differs decisively: reference backend -> QEMU 16.7% CPU
      (healthy/idle); my backend -> QEMU 96% CPU (a real spin my
      backend provokes). So there IS a backend bug, isolated now.
- RHEL qemu-kvm has NO vhost-user trace events compiled (--trace help
  lists none; -D writes nothing), so QEMU-side tracing is unavailable
  here -- the reference-backend diff is the right oracle instead.
Failed/learned:
- Don't trust a single success signal (serial) -- the reference oracle
  showed it was measuring the wrong thing. Banked.
- More && chains silently broke on non-zero pgrep/grep exits (no output,
  no commit); ran pieces separately. The pidfile/pgrep-in-chain family
  of traps is now costing real time -- switching to explicit loops and
  separate verification commands.
Metrics: ~45 min (timeboxed at the reference-oracle result).
Fork movement: F7 candidate (b) advancing; still OPEN (needs in-guest
  fio numbers, blocked on the spin bug + a real success signal).
Next: (a) get a real success signal -- boot the reference-backend config
  to SSH (add console=ttyS0 / try -nographic, or longer wait) so
  "working" is observable; (b) find my backend's spin -- likely the
  kick-eventfd read loop or a call-eventfd storm; diff my on-wire
  behaviour against qemu-storage-daemon. Consider the ublk candidate as
  a lower-friction path to the same F7 numbers (guest kernel 6.12 has
  ublk).

## 2026-07-28 04:40 -- F7 QEMU SMOKE: NEGOTIATION FULLY WORKS; MEM-TABLE
##                     REMAP-AFTER-START WEDGE (SPIRAL-RULED, BANKED)
Plan: drive the vhost-user backend from real QEMU (P-1.9 candidate b).
Done:
- vhost-user-blk-serve raw-file command (viperblock commit d7dc4f9,
  pushed). Host QEMU 7.2 confirmed to have vhost-user-blk-pci.
- Real-QEMU smoke test: the FULL vhost-user handshake completes -- with
  per-request logging I confirmed QEMU sends and my backend handles, in
  order: GET_FEATURES, GET/SET_PROTOCOL_FEATURES, SET_OWNER,
  SET_VRING_CALL/ERR, GET_CONFIG, SET_FEATURES, SET_MEM_TABLE,
  SET_VRING_NUM/BASE/ADDR, SET_VRING_KICK, SET_VRING_ENABLE. Every
  message accepted; QEMU proceeds through device realize.
Failed/learned (spiral rule applied -- third sub-problem, banked and
  stopped):
- After enable, QEMU issues a SECOND SET_MEM_TABLE and then busy-loops
  at 96% CPU with an EMPTY guest serial. Root-cause hypothesis (strong):
  the service goroutine started at the first KICK holds VirtQueue slices
  into the ORIGINAL mmap; the remap is not applied to the live queue.
  Fix next session: on SET_MEM_TABLE while running, re-map the vrings
  (and munmap old regions); confirm with QEMU -trace 'vhost_user*'.
- pidfile trap, AGAIN: my launch subshell's $! is not the server's pid;
  two commits silently no-op'd because a leftover vhost-serve held
  state / and a multi-line -m in an && chain mangled. Lesson tripled:
  use pgrep -x / pkill -f by name, and commit with a single-line -m in
  chained commands. Machine verified clean (no qemu/serve).
Metrics: negotiation reaches enable in <1 s; smoke debug ~30 min
  (timeboxed).
Fork movement: F7 candidate (b) prototype building; stays OPEN pending
  the in-guest fio numbers.
Next: mem-table remap-after-start fix -> guest boot -> in-guest fio
  vhost-vs-NBD (the F7 decision).

## 2026-07-28 03:55 -- F2 SLICE 2: PEER REPLICATION WIRED INTO THE BARRIER,
##                     REPLICA WAL BYTE-IDENTICAL TO PRIMARY
Plan: build the F2 replication transport and wire it into the engine.
Done:
- walrepl package (viperblock commit a298ca7, pushed): replica server
  (group-commit fsync, cumulative seq acks, handshake carries volume +
  WAL header so replica files are VALID WAL FILES -- future promotion
  can recover from them directly); ordered streaming client with a
  Barrier() that resolves only when everything sent is replica-durable.
- Engine wiring: WriteWAL streams each record under the WAL lock
  (replica order == file order); Flush() barrier = local fsync
  (SyncOnFlush) + replica durability; broken replica FAILS the barrier.
  Sharded WAL explicitly rejected with Replicator set (no single record
  order); legacy WAL is the serving default.
- Tests: transport byte-exactness (100 random-size records), empty-
  barrier fast path, dead-replica barrier failure (deterministic fake
  replica), and the integration crown jewel -- replica WAL file byte-
  identical to the primary's active WAL after a real VB flush barrier.
  FULL suite green (viperblock 180.5 s, all packages).
- Honest design note banked in the gate: replication currently covers
  FLUSHED writes; the acked-but-unflushed memory window (14.2% measured
  in P-1.3) is NOT yet replicated -- closing it needs replicate-on-
  WriteAt (streaming acks) or WAL-on-ack, the next design point.
Failed/learned:
- First dead-replica test closed the LISTENER and expected the barrier
  to fail -- but established connections outlive listeners; a
  deterministic fake replica (accept, read, hang up) is the right shape.
Metrics: slice ~620 lines incl. tests; suite 184 s.
Fork movement: F2 DECIDED -> first two implementation slices landed.
Next: QEMU vhost-user smoke test (device confirmed available in host
QEMU); then replicate-on-WriteAt design; then promotion slice.

## 2026-07-28 03:05 -- F2 ACKED BY HUMAN; PHASE 1 OPENED WITH THE DURABLE
##                     FLUSH BARRIER (P1.4 SLICE 1)
Plan: record the human's F2 ack and start Phase 1 with the thinnest
verifiable slice.
Done:
- F2 DECIDED: human ack verbatim in DECISIONS.md ("Lets go the way
  you're leaning with F22 - (b) sounds best, and now we can keep
  moving!"). Synchronous peer WAL replication is the architecture;
  Phase 1 implementation unblocked.
- P1.4 slice 1 LANDED (viperblock feat/replicated-wal-durability commit
  f2f267d, pushed): SyncOnFlush option -- Flush() fsyncs the active WAL
  (or dirty shards) via a new error-returning syncWALForBarrier before
  acknowledging the barrier; enabled in the nbdkit plugin (guest fsync
  now power-loss durable on the serving path). Behavioural tests assert
  the dirty-flag contract with the background syncer disabled; FULL
  engine suite green on the branch (viperblock 181.6 s, all packages).
- Design choice recorded in the flag's doc comment: opt-in rather than
  default-on so embedders keep their timing; serving entrypoints enable
  it; F2 replication extends the same barrier hook.
Failed/learned: none this slice.
Metrics: slice ~140 lines incl. tests; full suite 187 s wall.
Fork movement: F2 LEANING -> DECIDED (human ack).
Next: F2 slice 2 -- the replication transport (peer daemon speaking the
group-commit protocol prototyped in proto/f2walrepl, wired into
syncWALForBarrier); strace re-verification of the served path once the
long-drain chain frees the cluster.

## 2026-07-28 02:10 -- VHOST-USER-BLK BACKEND WORKS END-TO-END IN-PROCESS
Plan: continue the decision-free frontier while F2 waits (human query
02:36 confirmed: keep moving).
Done:
- vhost-user-blk backend daemon implemented (viperblock
  feat/data-path-vhost-user-blk commit fb656eb, pushed): full message
  loop, memfd mmap of guest regions, eventfd kick/call, single-queue
  service goroutine, virtio-blk IN/OUT/FLUSH/GET_ID against a pluggable
  BlockEngine interface (satisfied by *viperblock.VB and by test fakes).
- Functional test that plays QEMU's role IN-PROCESS: full vhost-user
  negotiation over a unix socket with a real memfd, real split rings,
  real eventfds; then write-4KiB / read-back-verify / flush-reaches-
  engine. Green first run, 31 ms. No QEMU needed for this layer.
- Long-drain race chain interim: pre-fix leg 0/10 hits even with
  faithful 400 MB bursts -- the wild race needs rarer timing; post-fix
  leg finishing ~03:00 (tracked). Verification conclusion from 20:15
  stands unchanged.
Failed/learned: nothing new; the codec/ring layers from the previous
  slice needed zero changes to support the daemon -- the synthetic-
  memory unit tests carried their weight.
Metrics: backend + test ~700 lines; functional test 31 ms.
Fork movement: F7 build advancing (P-1.9 remainder needs the in-guest
  fio comparison next).
Next: wire *viperblock.VB as the engine + a serve command; QEMU smoke
  test in the rig VM (vhost-user-blk-pci + memory-backend-memfd); then
  in-guest fio NBD-vs-vhost (ticks P-1.9, decides F7).

## 2026-07-27 20:15 -- RACE FIX VERIFICATION CLOSED HONESTLY: REPRODUCER
##                     TOO WEAK TO DIFFERENTIATE; FIX STANDS ON MECHANISM
Plan: process the pre-fix race leg results.
Done:
- Valid pre-fix leg: 30 write(4M)+reconnect iterations, 0 hits. So the
  cheap reproducer cannot trigger the race even WITHOUT the fix -- its
  close-side drain window is milliseconds, while the wild occurrence
  followed a ~400 MB burst (seconds-wide drain). Recorded plainly
  (viperblock fix/nbd-close-open-race commit 9907f0e): the fix is NOT
  differentially validated; it stands on the code-evident mechanism,
  fix-by-construction (one mutex), and 60 clean post-fix iterations.
  A faithful long-drain reproducer (~5 min/iteration) is queued as
  nice-to-have.
- Cleanup: the earlier leftover s3d (ignored SIGTERM) and the rerun's
  nbdkit (pidfile held the subshell pid -- recurring pattern) needed
  kill -9; machine verified clean.
Failed/learned: nbdkit's -P pidfile is written by nbdkit itself, but my
  scripts captured $! of the launching subshell instead in two places;
  when scripting nbdkit, always kill via ITS pidfile, and verify with
  pgrep -x afterwards.
Metrics: pre-fix leg 2h00 for 30 iterations (cluster start to stop).
Fork movement: none.
Next: morning report; vhost-user daemon loop (message loop + eventfd
kick/call + engine wiring).

## 2026-07-27 18:20 -- OVERNIGHT CHAIN RESULTS: P-1.7 FROZEN (123/621/94);
##                     RACE PRE-FIX LEG INVALID (FIPS BUILD FLAG), RERUNNING
Plan: process the overnight chain completion.
Done:
- P-1.7 TICKED [x]: full ceph/s3-tests baseline complete in 35m31s:
  123 passed / 621 failed / 94 skipped of 838. Area breakdown:
  sse/encryption 185, acl 59, multipart 54, policy 42, lifecycle 35,
  versioning 24, cors 14. Frozen with the full pass/fail list:
  predastore tools/s3tests/results/2026-07-27_baseline_counts.txt
  (commit 72c6269, pushed). This is the Phase 2 P2.4 floor.
- Race verification: POST-fix leg valid (60 iterations, 0 hits, ~2 min
  per iteration -- reconnect recovery against predastore is slow, itself
  worth noting). PRE-fix leg INVALID: the plugin I built from main
  crashed at startup -- built without GOFIPS140=v1.0.0, and viperblock's
  fipsboot guard refuses to run (trap banked: ALWAYS build the plugin
  via the Makefile target or set GOFIPS140 explicitly). Rebuilt with the
  flag, verified loadable with --dump-plugin, and a tracked rerun chain
  (30 iterations + cluster start/stop) is in flight.
- Cleanup: one leftover s3d killed; machine otherwise clean.
Failed/learned: GOFIPS140=v1.0.0 is REQUIRED at build time for any
  binary embedding viperblock (fipsboot init panics otherwise); the
  overnight chain should have smoke-checked the plugin loads before
  depending on it (now done in the rerun script).
Metrics: baseline 35m31s; postfix race leg 2h00 for 60 iterations.
Fork movement: none.
Next: pre-fix race count -> finish fix verification note; morning
report; vhost-user daemon loop.

## 2026-07-27 16:15 -- OVERNIGHT SESSION ARMED; VHOST-USER SKELETON LANDED
Plan: human going to bed; maximize unattended progress (protocol 09).
Done:
- s3-tests v2 "crash" triaged: MY setup bug -- --timeout-method=thread
  aborts the WHOLE pytest run on the first hung test (died 11:34 after
  7 min); AND the completion watcher never fired because its pgrep
  pattern matched its own command line (self-match trap, second bite).
  Both banked. Cluster itself healthy.
- Overnight chain launched as a TRACKED task (/tmp/overnight_chain.sh,
  log /tmp/overnight/chain.log): (1) s3-tests baseline v3 with SIGNAL
  per-test timeouts, 3 h cap; (2) NBD race verification, pre-fix plugin
  (built from main into /tmp/nbdkit-viperblock-plugin-PREFIX.so) vs
  post-fix, 60 write+reconnect iterations each; (3) cluster shutdown +
  orphan sweep. Sentinel /tmp/overnight/CHAIN_DONE.
- P0.2 groundwork: one-command regression runner written
  (project_management/tools/run_regressions.sh): six suites, one
  PASS/FAIL/SKIP line each + tally; predastore suite auto-skips when a
  dev cluster is up (fixture port collision guard). First validation
  run launched as a tracked task; results to journal on completion.
- F7 first code slice LANDED (viperblock branch
  feat/data-path-vhost-user-blk commit a783271, pushed): vhost-user
  control-plane codec (fd passing, mem-table, user->GPA translation) +
  split-virtqueue parsing (avail pop, chain walk with loop detection,
  used publish), amd64 memory-ordering documented; 5 unit tests green
  against synthetic guest memory.
Failed/learned: pytest-timeout thread method = run-abort (use signal);
  the pgrep self-match trap now has TWO scalps -- watchers must match on
  file sentinels, never process names.
Metrics: vhostuser slice ~500 lines with tests, unit suite 6 ms.
Fork movement: none (F2 still awaiting human ack; F7 in build).
Next (morning): read chain + runner results; freeze P-1.7 counts;
record race pre/post numbers into the fix commit's branch; continue
vhost-user daemon loop (kick/call eventfds + engine wiring).

## 2026-07-27 14:45 -- IN-GUEST RIG LIVE; NBD LIFECYCLE RACE FIXED
##                     (VERIFICATION QUEUED BEHIND THE BUSY CLUSTER)
Plan: work that neither blocks on nor disturbs the running s3-tests.
Done:
- s3-tests completion watcher armed as a tracked background task (the
  run itself was detached, so no automatic notification would have
  fired -- trap noted). Human does not need to ping.
- In-guest rig COMPLETE as substrate: Debian 13 guest (kernel 6.12,
  ublk-capable) boots under /usr/libexec/qemu-kvm with KVM accel, no
  root: cloud-init NoCloud seed with SSH key, user networking
  (hostfwd 2222->22), passwordless sudo, fio 3.39 + nbd-client
  provisioned, clean poweroff; provisioned overlay kept at
  ../vm-images/rig-node1.qcow2 for instant reuse. No /dev/kvm inside
  guest (add -cpu host when nesting is needed). Serves P-1.2, the
  F7/P-1.9 in-guest bench, and P1.4 verification.
- NBD lifecycle race ROOT-CAUSED and FIXED (viperblock branch
  fix/nbd-close-open-race, commit 6a871d2, pushed): the plugin's Close
  ends in vb.Close() which REMOVES the volume's local tree; nbdkit can
  run the next connection's Open while the previous Close is
  mid-teardown (CanMultiConn=false bounds connections, not callback
  overlap), so the new connection's WALs/checkpoints get deleted under
  it -- the exact ENOENT seen during the P-1.5 bench, and a silent
  durability hazard (unlinked-but-open WAL). Fix: a lifecycle mutex
  ordering Open after any in-flight Close/Unload. Repro script
  committed (tests/nbd-race/nbd-race-repro.sh).
  HONESTY NOTE: verification is PENDING -- the repro needs the
  predastore cluster, which is serving the s3-tests baseline; run the
  pre/post-fix counts when it frees. The commit message says so.
Failed/learned:
- Detached (shell-&) background jobs produce NO completion
  notification; use tracked tasks or arm a watcher. Banked.
Metrics: VM first boot to SSH ~100 s; apt provision ~150 s.
Fork movement: none.
Next: when s3-tests completes -- freeze counts (P-1.7), verify the race
fix pre/post; then vhost-user-blk skeleton per the design note.

## 2026-07-27 13:40 -- P-1.7: STOCK S3-TESTS CANNOT EVEN CLEAN UP AFTER
##                     ITSELF AGAINST PREDASTORE (BASELINE FINDING #1)
Plan: get a meaningful per-test s3-tests baseline.
Done:
- First full run: 2 passed, 836 ERRORS in a cascade -- root-caused, not
  guessed: the suite's nuke_bucket needs ListObjectVersions (predastore
  answers it EMPTY, not with an error) + DeleteObjects (missing), so
  DeleteBucket hits BucketNotEmpty and every later test dies in setup.
  Both operations were already on the P2.3 gap list; the suite just
  turned them into a hard blocker. That is baseline finding #1.
- Wrote a cleanup-glue-only patch (nuke fallback: ListObjectsV2 +
  per-key DeleteObject; assertions untouched) -- oracle-rig
  accommodation in the FEMM-mesh-forcing tradition, documented in the
  patch header. Verified: previously-cascading tests now produce real
  pass/fail. Rig committed reproducibly (patch + conf + README) as
  predastore tools/s3tests/ (commit 8888530, pushed).
- Full run v2 in flight (60 s per-test timeouts, 4 h cap). Debian 13
  cloud image download started in background for the upcoming in-guest
  rig (serves P-1.2, P-1.9 bench, P1.4 verification).
Failed/learned:
- An unversioned store that answers ListObjectVersions with an empty
  200 (instead of NotImplemented) silently breaks clients that fall
  back on error -- worth fixing in predastore as part of P2.3 (return
  NotImplemented, or implement the listing shape unversioned).
Metrics: cascade root-cause ~20 min; patch + verify ~15 min.
Fork movement: none.
Next: freeze v2 counts; then in-guest rig.

## 2026-07-27 12:50 -- F2 DEFERRED BY HUMAN; POWER-LOSS MECHANISM PROVEN
##                     BY STRACE; S3-TESTS BASELINE LAUNCHED; F7 DESIGN
##                     NOTE WRITTEN
Plan: human deferred the F2 ack ("people asleep") and asked for maximal
independent progress. Frontier chosen: P-1.3 power-loss leg, P-1.7
s3-tests, F7 design note.
Done:
- P-1.3 power-loss leg CLOSED by open gate restructure: instead of a
  VM power-cut loss count, proved the mechanism at syscall level --
  strace on the NBD-serving process recorded ZERO fsync-family syscalls
  across 50 explicit NBD_CMD_FLUSH commands (only open-recovery fsyncs
  and one 200 ms background-syncer tick). Guest fsync is NOT power-loss
  durable. Artifact: viperblock commit a40cc27 (finding + raw strace).
  The loss-COUNTING rig (volatile-page-cache FUSE fs) is displaced to
  P1.4 where it will verify the fix. P-1.3 now [x].
- P-1.7 launched: ceph/s3-tests cloned, venv (py3.9) built,
  predastore-loopback.conf written; discovered the suite takes region
  from AWS_DEFAULT_REGION (not the conf) -- set to ap-southeast-2;
  smoke tests green; full 838-test run started in background with 60 s
  per-test timeouts. Caveat recorded: single credential set reused for
  alt/tenant, so cross-account tests are not meaningful in this
  baseline.
- F7 design note written (project_management/
  F7_VHOST_USER_BLK_DESIGN_NOTE.md): pure-Go vhost-user-blk backend in
  viperblockd, amd64-first, single-queue thin slice; Rust sidecar and
  cgo libvhost-user rejected (both reintroduce a per-op boundary);
  control-plane-only libvhost-user recorded as escalation; bench needs
  an in-guest fio rig (shared with P-1.2/P1.4 needs).
Failed/learned:
- qemu-io is a fine explicit-FLUSH NBD client for barrier experiments
  (qemu-img bench cannot issue flushes).
- s3-tests current tree keeps tests under s3tests/functional (not the
  older s3tests_boto3 path).
Metrics: strace experiment ~10 min end to end; s3-tests setup ~2 min.
Fork movement: F2 explicitly DEFERRED by the human (stays LEANING (b),
Phase 1 implementation blocked on ack); F7 unchanged (design note only).
Next: freeze the s3-tests counts when the run completes; then the
in-guest rig (serves P-1.2, P-1.9 bench, and P1.4 verification).

## 2026-07-27 11:55 -- P-1.9 CANDIDATE (a): GO NBD SERVER ON PAR WITH
##                     NBDKIT -- THE TAX IS THE PROTOCOL, NOT THE PROCESS
Plan: prototype fork-F7 candidate (a), a native Go NBD server around the
viperblock engine, and measure it on the identical rig as the nbdkit
numbers.
Done:
- ~400-line NBD server (fixed-newstyle negotiation with NBD_OPT_GO +
  legacy EXPORT_NAME, simple replies, 16-way worker dispatch,
  ErrZeroBlock -> zeroes semantics) on viperblock branch
  feat/data-path-native-nbd-server (commits 7a501af + 4c80a18).
  qemu-img negotiated with it on the first try.
- Measured (same predastore rig, artifact tests/perfbench/results/
  2026-07-27_go_nbd_server_vs_nbdkit.txt): write d1 16.1k IOPS vs
  nbdkit 15.2k (+6%); write d16 10.0k vs 12.1k (-17%); reads -10..-11%.
- CONCLUSION (decision-relevant for F7): eliminating the nbdkit process
  is NOT the performance fix -- any NBD server pays the same per-op
  socket round trip, and the engine write lock + backend reads dominate
  the rest. Candidate (a) remains worth shipping for operational
  reasons only. The performance endgame is vhost-user-blk (shared-
  memory virtio rings), a multi-session build. P-1.9 marked [~] with
  exactly that remaining.
Failed/learned:
- Committed unformatted Go on two branches (gofmt -l after, not before,
  the commit); fixed with style commits. Add gofmt to the pre-commit
  habit for these tool dirs.
Metrics: server written+negotiating in ~25 min; bench matrix ~5 min
  (read d1 dominates at 255.7 s).
Fork movement: F7 evidence extended; stays OPEN pending vhost-user-blk.
Next: F2 human ack; P-1.3 power-loss leg; P-1.7 s3-tests baseline;
vhost-user-blk prototype planning.

## 2026-07-27 11:05 -- P-1.5 COMPLETE: NBDKIT TAX MEASURED (2.5-3.8x);
##                     NO ROOT NEEDED FOR THE WHOLE RIG
Plan: build the NBD-path rig without root and measure the nbdkit tax
(P-1.5(ii), fork F7 evidence).
Done:
- nbdkit 1.38.5 + devel headers obtained WITHOUT root: dnf download +
  rpm2cpio extract to /tmp/nbdkit-root; wrote a corrected nbdkit.pc
  (prefix + missing Cflags) and built the viperblock nbdkit plugin with
  PKG_CONFIG_PATH pointing at it (make go_build_nbd, 33 MB .so).
- Transport tax isolated (qemu-img bench, 4 KiB, unix socket, same raw
  file): direct 86.2k read IOPS d1 / 298k d16 / 80.7k write d1; through
  nbdkit's own C file plugin 33.6k / 78.4k / 32.3k = 2.5-3.8x loss.
  The human's bottleneck call is confirmed with numbers.
- Production path (nbdkit + Go plugin + predastore 3-node loopback,
  volume seeded by new createvol-s3 helper): write d1 15.2k IOPS,
  write d16 12.1k (regresses with depth -- engine lock contention,
  matches the engine baseline), read d1 439 IOPS (2.28 ms/op,
  backend-dominated), read d16 5.1k. Artifact:
  tests/perfbench/results/2026-07-27_nbd_path_vs_direct.txt (commit
  5c0155f). P-1.5 ticked [x]; F7 evidence appended (fork stays OPEN for
  P-1.9 candidate prototypes).
Failed/learned:
- BUG (filed in STATUS backlog): reconnecting to the plugin after an
  unclean client disconnect triggers WAL recovery that dies on a
  missing local checkpoints directory ("open .../checkpoints/
  blocks.00000000.bin: no such file or directory") -- volume unserveable
  for that connection. Recovery path assumes a directory that fresh
  WAL base_dirs lack.
- Bit my own banked pkill trap AGAIN (pgrep -f "qemu-img bench" matched
  the wrapper shell): the trap entry now exists for a reason; use
  pgrep -x. Also: a daemonized nbdkit survived a pidfile kill (pidfile
  held the subshell pid) and two nbdkits briefly contended for the same
  socket path, causing a silent first-bench stall -- always verify with
  pgrep -x after kills.
Metrics: whole rig built + measured in ~45 min; nbdkit RPM download was
  the slowest step (~3 min of dnf metadata).
Fork movement: F7 evidence appended (stays OPEN pending P-1.9).
Next: P-1.9 native-Go-NBD + vhost-user-blk prototypes; P-1.3 power-loss
leg; P-1.7 s3-tests; F2 ack from human.

## 2026-07-27 09:20 -- P-1.4 DECIDES F2 (LEANING PEER REPLICATION);
##                     P-1.5 ENGINE BASELINE FROZEN
Plan: run the two measurement gates queued in STATUS: engine perf
baseline (P-1.5 part i) and the F2 durability prototype (P-1.4).
Done:
- perfbench (viperblock feat/crash-consistency-harness commit 19adcf3):
  engine-level bench through the production open sequence. Frozen CSV:
  tests/perfbench/results/2026-07-27_engine_baseline_xfs_ssd.csv.
  Headlines: randwrite-4k p50 5 us (pure memory ack) with MAX 175-420 s
  (writes block for MINUTES behind synchronous drains under sustained
  load -- a guest-visible freeze; new Phase 1 concern named alongside
  durability); flush barrier p50 8.9 ms / p99 100 ms at 4 MiB dirty;
  randread-4k 6.1k IOPS @1 worker, 92.6k @16; seqwrite-128k 117 MB/s.
- f2walrepl prototype (viperblock feat/replicated-wal-durability commit
  f7d5ca1): three candidate mechanisms measured, 4 KiB records:
  local fsync p50 5627 us == peer-replicated p50 5630 us (same run;
  replication is FREE relative to the fsync it requires; localhost RTT
  caveat recorded); predastore per-write PUT p50 77.6 ms @c1, 293 ms
  @c16, ~50 ops/s ceiling -> option (c) REJECTED. F2 moved to LEANING
  (b) synchronous peer WAL replication; human ack pending (MAJOR).
Failed/learned:
- Predastore loopback cluster: forgetting SSL_CERT_FILE at launch fails
  as "unknown authority" on the :6660 db API; recipe updated in
  ENVIRONMENT.md (bit twice today).
- Absolute disk-latency numbers on this shared host swing 3.5x with
  ambient IO; A-vs-B latency claims must be same-run (banked as a trap).
- perfbench's first f2 peer run overlapped the background baseline and
  polluted the numbers; reran quiet. Same lesson as above.
Metrics: perfbench full run ~24 min (prefill+drains dominate); f2
  prototype ~2.5 min local+peer, ~35 s s3put; predastore cluster
  start/stop ~40 s.
Fork movement: F2 OPEN -> LEANING (b). F7 unchanged (P-1.5(ii) NBD leg
  still needed for the nbdkit tax number).
Next: F7/P-1.9 rig (nbdkit via rpm-extract or source build, or straight
  to the native-Go-NBD + vhost-user-blk prototypes); P-1.3 power-loss
  leg; P-1.7 s3-tests baseline.

## 2026-07-27 07:50 -- STORAGE REPOS PUSHED; DNS FIX VERIFIED; ONE
##                     ATTRIBUTION CORRECTED
Plan: use the human's DNS fix + widened token; push storage branches.
Done:
- Human fixed the wildcard DNS (verified: NXDOMAIN behaves) and granted
  the token read+write on gitcnd/{spinifex,viperblock,predastore}.
- Fork API remains impossible for ANY fine-grained token here (the source
  repos are mulgadc-owned; "Resource not accessible by personal access
  token"). Created PLAIN repos gitcnd/viperblock + gitcnd/predastore via
  POST /user/repos instead and pushed: main, both feature branches each,
  and tag v1.13.0. All storage work is now off-machine. Plain repos
  cannot PR against mulgadc/*; swap to true forks later if upstreaming
  (noted in STATUS).
- DNS-fix retest of the two normal-mode unit failures:
  services/viperblockd PASSES now (2.7 s; DNS attribution CONFIRMED).
  spinifex/daemon TestClusterManager_TLSServesHTTPS still FAILS 3/3.
Failed/learned (attribution correction, per the honesty rule):
- My P-1.1 note claimed both failures were "caused by the machine's
  wildcard-DNS trap". WRONG for spinifex/daemon; corrected in the gate
  note. Investigation so far: standalone replica (TLS 1.3, the pinned PQ
  curve list X25519MLKEM768/SecP384r1MLKEM1024/X25519/P-384, ECDSA P-256
  cert, dangling plain-TCP conn like the test) handshakes in 2 ms on this
  host, so loopback TLS itself is fine; the 2 s stall is specific to the
  daemon test binary (its TestMain pins SPINIFEX_HOST_VCPU=4 and starts a
  shared NATS fixture). Passes in hermetic netns. Parked as an open
  non-blocking triage item (spiral rule) -- P-1.1's mode-union gate
  evidence still stands.
- Trap: `go mod init` in a scratch dir writes the SYSTEM go version
  (1.21) into go.mod, so newer TLS constants are undefined until
  `go mod edit -go=1.26.5`.
Metrics: pushes ~10 s; retests ~45 s.
Fork movement: none.
Next: P-1.4 WAL-replication prototype (F2 evidence).

## 2026-07-27 06:35 -- P-1.6 PREDASTORE SINGLE-NODE-KILL BASELINE:
##                     READS SURVIVE (2320x SLOWER), WRITES GO TO ZERO
Plan: stand up the 3-node predastore dev cluster on this host and measure
availability through a single-node SIGKILL (gate P-1.6).
Done:
- Cluster: 3node config rewritten to loopback IPs (127.0.0.1/2/3, no root
  needed), sudo shimmed to no-op (trust store + ip-addr-add skippable on
  Linux), TLS cert regenerated so SANs cover the loopback hosts
  (inter-node TLS verifies SANs). Cluster of 3 s3d processes healthy.
- Probe: cmd/cluster-availability-probe on predastore branch
  feat/shard-healer-and-read-repair (commit dbe31cf): 200 x 64 KiB
  deterministic self-verifying objects via the AWS SDK.
- MEASURED (full report committed next to the probe):
  - Healthy: write 200/200 in 6.02 s; read 200/200 in 0.82 s.
  - kill -9 node 3: read 200/200 (RS 2+1 reconstruction works) but in
    1907.56 s (~2320x; ~9.5 s/object of dead-node timeouts).
  - Writes during outage: 0/10 ("quic dial 127.0.0.3:9991: timeout");
    placement requires all K+M nodes -- no degraded write path.
  - CreateBucket during outage: fails ("all nodes failed"; the bucket
    metadata lookup insists on the dead node despite Raft quorum).
  - Node rejoin (data intact): reads back to 0.84 s.
  - Nothing restores redundancy automatically (healer unimplemented,
    matches predastore docs/TODO.md).
- Implication for Phase 2 scope: the healer alone is NOT enough; failure
  detection + read shortcutting + degraded/quorum writes + metadata-read
  failover are all required for an EBS-grade backing store. Phase 2 gates
  should be extended accordingly when Phase 2 is opened.
Failed/learned:
- pkill -f with a pattern contained in my own command line killed my own
  shell (agent wrapper embeds command text) -- banked in ENVIRONMENT.md.
- start.sh reuses an existing /tmp/predastore/server.pem; stale SANs from
  a previous config produce inter-node x509 SAN failures -- delete the
  cert when changing host IPs.
Metrics: outage read study 31.8 min wall; outage write study 3 min.
Fork movement: none directly; strengthens F1's premise (stack hardening).
Next: P-1.4 WAL-replication prototype (F2 evidence); P-1.3 power-loss
leg; extend Phase 2 gate list per the implication above.

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
