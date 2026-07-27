# Preliminary Security Hardening Review -- Spinifex + Viperblock + Predastore

Status: PRELIMINARY, READ-ONLY (2026-07-27). No code was changed. This
document is a handoff for a later agent to investigate each item deeply
and rectify. It is a defensive review of OUR OWN code aimed at hardening
the stack before wider release; language is deliberately neutral.

Method: three parallel read-only code reviews (one per repo) plus manual
spot-checks, covering authentication, authorization, input validation,
secret handling, transport security (TLS), the web console, encryption at
rest, and resource-exhaustion safety. Tests and clusters were NOT run
(another workstream was using them); this is static code inspection only.

Reviewed at:
- spinifex: /home/devnull/Downloads/github/spinifex (branch main + working tree)
- viperblock: /home/devnull/Downloads/github/viperblock (v1.13.0 + local branches)
- predastore: /home/devnull/Downloads/github/predastore (v1.13.0 + local branches)

IMPORTANT CAVEAT: findings are from static reading and have NOT been
reproduced at runtime. Each needs confirmation (a focused test or a
controlled local repro) before and after any fix. Treat severities as
provisional. Note where a subsequent look finds a compensating control.

Source review agents (for the deeper-dive agent to resume with more
context): spinifex [security review](3c14e554-196b-473b-8b63-74cd82c25b3b),
viperblock [security review](877504cf-0748-4c6b-86d6-b6855e064474),
predastore [security review](42c6cace-86ea-4db3-9bcf-be4bda9404c1).

---

## How to use this document

1. Start with the "Remediation priority" list at the end -- it is ordered
   by (severity x blast radius x ease of confirmation).
2. For each item: reproduce it first (write the failing test / repro),
   THEN fix, THEN re-run the repro, per the project methodology
   (no validation artifact, no claim).
3. IDs are stable within this document (SP-* spinifex, VB-* viperblock,
   PD-* predastore, OPS-* operational). Cite them in commits and the
   journal.
4. Anything here that changes product scope or an intended trust boundary
   (e.g. "the UI is not an auth boundary") is a NEEDS-HUMAN decision, not
   a unilateral change.

---

## Operational / hygiene (not product-runtime, but do first)

### OPS-1 -- Live access token present in the working tree  [HIGH]
- Where: `spinifex/.env` (lines 1-2). Confirmed gitignored and NOT in git
  history (`git log --all -- .env` empty), so it did not leak via the repo.
- Concern: a live fine-grained token sits in plaintext on disk in the
  audited tree; exposure risk is via backups, shared-host access, or an
  accidental copy outside the ignore rule. (Token value intentionally not
  reproduced in this document.)
- Direction: treat the token as compromised and rotate/revoke it; inject
  such tokens at runtime from a secret store or CI secret rather than a
  checked-out file. This is the token the automation uses to push to the
  gitcnd/* forks, so coordinate rotation with that workflow.

---

## Spinifex (control plane / gateway / UI)

The SigV4 signature path and the shared IAM evaluator are the strong
parts (see "Confirmed-safe" below). The gaps are mostly at the edges of
authorization and in operational surfaces.

### SP-1 -- STS actions are not gated by the caller's identity policy  [HIGH]
- Where: `spinifex/gateway/sts.go` ~101-103; also `GetSessionToken` handler.
- Evidence: comment "STS actions are not gated by caller IAM policy ... so
  no checkPolicy pass runs here."
- Concern: an authenticated principal that matches a role's trust policy
  can assume the role even with no `sts:AssumeRole` permission of its own.
  AWS requires BOTH the trust policy AND the caller's identity permission.
  `GetSessionToken` is similarly ungated.
- Direction: run the identity-policy check (`checkPolicy(r,"sts",action)`)
  before the STS handler, keeping the trust-policy/JWT check as the second
  gate. Confirm with a test: a principal with an empty identity policy
  must be denied AssumeRole even when the trust policy allows it.

### SP-2 -- Identity-policy `Condition` blocks are silently dropped  [HIGH]
- Where: policy `Statement` type has no `Condition` field
  (`predastore/pkg/iampolicy/types.go` ~22-27, used by spinifex);
  `spinifex/handlers/iam/service_impl.go` `ValidatePolicyDocument` ~1858.
- Evidence: the struct carries only `Effect`/`Action`/`Resource`;
  validation never inspects or rejects a `Condition`.
- Concern: a policy that scopes an `Allow` with a `Condition` (MFA present,
  source range, secure transport, etc.) is stored and evaluated as an
  UNCONDITIONAL allow -- broader than the author intended. A conditional
  `Deny` is likewise dropped. This silently widens least-privilege
  policies.
- Direction: either implement condition evaluation, or -- as an interim,
  fail-closed measure -- reject any policy document containing a
  `Condition` (and `NotAction`/`NotResource`) at write time, so intent is
  never silently discarded. Note trust policies already reject unsupported
  conditions at write time (`roles.go` ~698-718) -- mirror that.

### SP-3 -- `sts:ExternalId` confused-deputy protection is accepted but not enforced  [MEDIUM]
- Where: `spinifex/handlers/sts/assume_role.go` (external id only logged,
  ~105; `evalTrustPolicy` ~197-237 matches Effect/Action/Principal only).
- Concern: a trust policy that relies on an external id to stop a third
  party from assuming a role gets no actual enforcement here. This is the
  concrete trust-path consequence of SP-2.
- Direction: enforce the external id against the trust policy condition
  before issuing the session; fold into the SP-2 condition work.

### SP-4 -- Fail-open branches in the resource authorization check  [HIGH]
- Where: `spinifex/gateway/gateway.go` ~420-445 (`checkPolicyResource`).
- Evidence: returns `nil` (allow) when the IAM service is unset, when
  there is no auth context, or when identity is empty -- described as
  "pre-IAM compatibility".
- Concern: a misconfigured/partly-initialized gateway, or a request that
  reaches this point without an identity, is authorized unconditionally.
  Note the sibling `evaluatePrincipalPolicy` (~474-480) fails CLOSED on a
  nil IAM service -- the asymmetry is the risk.
- Direction: fail closed (AccessDenied/InternalError) by default; if a
  compatibility mode is genuinely required, gate it behind an explicit,
  off-by-default flag rather than a silent default.

### SP-5 -- Object-store secret passed on the nbdkit command line  [MEDIUM]
- Where: `spinifex/nbd/nbd.go` ~66-72, 95 -- `fmt.Sprintf("secret_key=%s",
  ...)` into `exec.Command("nbdkit", args...)`.
- Concern: the backend secret is visible in process listings
  (`/proc/<pid>/cmdline`) to other local users/processes on the node.
- Direction: pass secrets via environment or a mode-restricted file;
  keep them out of argv and out of any arg logging. (Viperblock side of
  the same issue is VB-4/VB-config-creds.)

### SP-6 -- Unbounded body read on the anonymous STS path  [MEDIUM]
- Where: `spinifex/gateway/sts_anonymous.go` ~33-38 -- `io.ReadAll(r.Body)`
  with no size cap before signature/anonymous-action filtering.
- Concern: an unauthenticated POST can drive a large allocation before any
  gate runs (memory-exhaustion risk).
- Direction: wrap with `io.LimitReader` aligned to the SigV4
  `MaxPayloadLen` (10 MiB).

### SP-7 -- Gateway HTTP server missing read/write/idle timeouts  [MEDIUM]
- Where: `spinifex/services/awsgw/awsgw.go` ~369-378 (only
  `ReadHeaderTimeout` + TLS set).
- Concern: slow or stuck clients can hold connections open; the UI server
  sets all three timeouts but the gateway does not.
- Direction: set `ReadTimeout`/`WriteTimeout`/`IdleTimeout` (mirror the UI
  server) and consider `MaxHeaderBytes`.

### SP-8 -- Proxy-to-backend TLS has no minimum version  [MEDIUM]
- Where: `spinifex/services/spinifexui/proxy.go` ~31-34 -- `tls.Config{
  RootCAs: pool}` with no `MinVersion`.
- Concern: the UI-to-backend hop can negotiate below the TLS 1.3 floor the
  listeners enforce.
- Direction: set `MinVersion: tls.VersionTLS13` and the shared curve list.

### SP-9 -- Certificate verification disabled on several internal probes  [MEDIUM]
- Where: `spinifex/daemon/peer_health.go` ~61;
  `spinifex/handlers/eks/cluster_reconciler.go` ~355;
  `spinifex/lbagent/healthcheck.go` ~67 (`InsecureSkipVerify: true`).
- Concern: peer/K3s/NLB health probes do not pin a CA, so on-path
  tampering could falsify health signals or observe probe traffic. Scope
  is health-checking, not the main API, and the code comments already flag
  it as pending hardening.
- Direction: pin the cluster CA or known fingerprints (as the comments
  intend).

### SP-10 -- UI stores session credentials in localStorage  [MEDIUM]
- Where: `spinifex/services/spinifexui/frontend/src/lib/auth.ts` ~19-65.
- Concern: short-lived STS session credentials in `localStorage` are
  readable by any injected script or malicious browser extension; a single
  content-injection issue becomes credential disclosure. (This is not a
  classic cross-site request issue -- SigV4 needs the secret -- the risk
  is script-injection reads.)
- Direction: prefer in-memory storage or an httpOnly-cookie + backend-for-
  frontend pattern; keep the CSP tight (already present) and TTLs short
  (already STS-based). Product-shape decision -> flag for human.

### SP-11 -- Console exposure and unauthenticated helper routes  [MEDIUM/INFO]
- Where: `spinifex/services/spinifexui/spinifexui.go` ~54-58, 195-212 --
  default bind `0.0.0.0`; `/api/ca.pem` served without auth; `/proxy/*`
  reverse-proxies relying entirely on backend SigV4.
- Concern: the console and CA download are reachable on all interfaces by
  default; the proxy is only safe if EVERY upstream route requires SigV4.
- Direction: default-bind to localhost or a documented management network;
  confirm no unsigned upstream route is reachable through `/proxy/*`;
  document explicitly that the UI is not itself an auth boundary.

### Spinifex -- warrants deeper review (not confirmed defects)
- SP-D1 QEMU arg pipeline: `vm.go` ~420-469 builds `exec.Command` from
  `VMConfig`. It is an arg slice (not a shell), so there is no shell-
  interpolation issue; confirm that no EC2-API-supplied field (device
  strings, kernel cmdline, paths) reaches these args without allowlisting.
- SP-D2 OIDC discovery/JWKS public endpoints (IRSA): review key rotation
  and whether cluster enumeration via path parameters is acceptable.
- SP-D3 ECR `/v2/*` JWT bridge mounted outside SigV4: spot-check token
  validation and policy rehydration for any fail-open branch.
- SP-D4 installer `firstboot` script generation (`fmt.Sprintf` into a bash
  script): confirm no externally-influenced value is interpolated.

### Spinifex -- confirmed-safe (do not chase)
- SigV4 signature compare is constant-time (`hmac.Equal`); session-token
  compare uses `subtle.ConstantTimeCompare`.
- Clock skew enforced (15 min); skewed requests become auth failures.
- IAM evaluator is default-deny (no match -> Deny; explicit Deny wins);
  root bypass is scoped to the global root user only.
- JWT paths pin the algorithm (ES256), require expiry, validate audience;
  JWKS fetched from internal NATS, not a token-supplied URL.
- Auth rate-limit key uses `RemoteAddr` only (no spoofable forwarded-for).
  BUT see the note below on proxy interaction.
- TLS 1.3 minimum on the gateway and UI listeners; UI sets CSP, HSTS,
  nosniff, frame-ancestors 'none'.
- KeyPair name validation rejects traversal sequences.
- No `database/sql` in production paths -> no classic SQL-injection surface.

### Spinifex -- note carried from the earlier pass (verify against SP-11)
- Auth lockout keying vs the UI proxy: because the UI proxy rewrites the
  origin, browser-originated auth attempts may share one rate-limit bucket
  and a single success may reset the counter. Worth confirming once SP-11
  bind/forwarding is settled; the fix is to trust a forwarded-for only
  from the vetted loopback proxy and to also throttle per presented
  access-key id.

---

## Viperblock (block storage engine + nbdkit plugin)

The encryption design is careful and fail-closed (see confirmed-safe).
The most serious item is a durability/data-loss path on Close, plus the
usual wire/parameter validation and secret-in-log items.

### VB-1 -- Close can delete unrecovered WAL after a failed chunk upload  [CRITICAL]
- Where: `viperblock/viperblock.go` ~4976-5029.
- Evidence: logs "Could not Write WAL to Chunk during Close, proceeding to
  save block state" and still calls `RemoveLocalFiles()` when the drain
  errored.
- Concern: if the forced WAL->chunk upload fails, Close may still persist a
  block map that omits those blocks and then delete the local volume tree,
  so unrecovered WAL records are lost and cannot be replayed on next open.
  This is a durability defect with a security-adjacent impact (silent data
  loss). NBD Close/Unload call DrainToBackend then Close on the same path.
- Direction: fail closed -- never `RemoveLocalFiles` when any drain/chunk/
  checkpoint step failed; keep local WAL + checkpoints until chunks and the
  live checkpoint have all succeeded. Confirm with a fault-injection test
  (backend PUT fails during Close -> local WAL retained -> next open
  recovers). Coordinate with the reconnect-lifecycle fix already on
  `fix/nbd-close-open-race`.

### VB-2 -- Volume names are not sanitized before path/key construction  [HIGH]
- Where: `types/types.go` ~69-72; `viperblock/backends/file/file.go`
  ~126-147 (names interpolated into local dirs, backend keys, WAL paths,
  the snapshot socket path).
- Concern: a volume name containing `..` or path separators is interpolated
  into filesystem paths and object keys, allowing reads/writes outside the
  intended volume prefix. Whether a name can be attacker-influenced depends
  on the spinifex layer (volume IDs are server-generated today), so this is
  defense-in-depth at the engine boundary.
- Direction: validate at the engine boundary -- reject empty names or names
  containing `/`, `\`, `..`, NUL; ideally allow only `[A-Za-z0-9._-]`.
  `filepath.Clean` + `HasPrefix(resolved, baseDir+sep)` for local paths.

### VB-3 -- NBD Zero allocates a request-controlled size  [HIGH]
- Where: `nbd/viperblock.go` ~480 -- `make([]byte, count)` where `count` is
  the NBD Zero length (up to ~4 GiB).
- Concern: a single Zero request can drive a multi-GiB allocation before
  bounds checks run (memory-exhaustion risk). Partly mitigated if an
  nbdkit blocksize filter is always present.
- Direction: cap `count`; zero in bounded chunks rather than one big
  allocation; do not depend on an external filter being configured.

### VB-4 -- S3 config (including secret) logged on Open  [HIGH]
- Where: `nbd/viperblock.go` ~268 -- `slog.Info("Creating Viperblock
  backend with btype, config", cfg)` where `cfg` includes `SecretKey`.
- Concern: backend secret material can land in structured logs (and OTLP
  if configured).
- Direction: log only non-secret fields; add a redacting `LogValue()` to
  `S3Config` so the secret cannot be logged by accident anywhere.

### VB-5 -- `ReadWAL` allocates from an unvalidated on-disk length  [MEDIUM]
- Where: `viperblock/viperblock.go` ~2193-2197 -- `block.Len` read from the
  header then `make([]byte, block.Len)`. The production recovery path uses
  fixed sizes, but `ReadWAL` (used by `cmd/sfs`) does not.
- Direction: require `block.Len <= BlockSize`; use fixed-size reads as the
  recovery path does; reject oversized values before allocating.

### VB-6 -- Snapshot control socket has no peer authentication  [MEDIUM]
- Where: `nbd/viperblock.go` ~355-390 -- `net.Listen("unix", sockPath)`;
  any connection triggers `DrainToBackend()`.
- Concern: any local user that can reach the socket can force repeated
  expensive drains (local resource pressure). The path also embeds the
  unsanitized volume name (VB-2).
- Direction: restrict socket mode/ownership (0600, service user); optional
  peer-credential allowlist; require a shared nonce on the first line
  before acting.

### VB-7 -- TRIM/discard is a silent no-op  [MEDIUM, correctness]
- Where: `nbd/viperblock.go` ~495-499 -- `Trim` logs and returns nil.
- Concern: the guest believes discard succeeded; space and block-map
  entries are not reclaimed, so capacity accounting and GC diverge from
  guest state.
- Direction: implement discard (clear map + GC) or advertise
  `CanTrim=false` until it is implemented.

### VB-8 -- Weak default base directory (`/tmp/viperblock`)  [MEDIUM]
- Where: `viperblock/viperblock.go` ~983-984.
- Concern: on multi-user hosts `/tmp` invites symlink races and cross-user
  visibility for WAL/checkpoints. NBD already requires an explicit BaseDir;
  the risk is other embedders taking the default.
- Direction: require an explicit BaseDir in production; refuse a `/tmp`
  default when encryption is enabled.

### VB-9 / VB-10 -- WAL read short-read + sharded-WAL torn-write handling  [LOW]
- Where: `viperblock.go` ~2825-2848 (short read before CRC on the
  unencrypted path; recovery + encrypted paths correctly use
  `io.ReadFull`); ~2157-2167 (sharded WAL does not truncate to the prior
  boundary on a torn write, unlike the legacy path).
- Direction: use `io.ReadFull` and treat `ErrUnexpectedEOF` as end-of-valid
  records; mirror the legacy pre-stat + truncate-on-short-write in the
  sharded path.

### VB-11 -- Cache size accepted without an upper bound  [LOW]
- Where: `nbd/viperblock.go` ~143-148; `SetCacheSize` ~880-914.
- Concern: a misconfigured `cache_size` allows resident growth up to
  `cache_size * BlockSize`. (The write buffer itself is bounded at 256 MiB
  -- that part is sound.)
- Direction: cap `cache_size` (absolute + % of RAM); reject negative /
  overflowing values.

### VB-12 -- `StateBody` returns unauthenticated metadata (API footgun)  [INFO]
- Where: `viperblock/crypto.go` ~195-203 -- documented as not verifying the
  auth tag. The hot open path (`LoadStateRequest`) does verify when
  encryption is on.
- Direction: make the unauthenticated nature obvious in the name; ensure
  spinifex never makes a security decision on `StateBody` alone.

### Viperblock -- deeper review
- VB-D1 plaintext block-to-object checkpoints (CRC only): on encrypted
  volumes a wrong remap should fail AEAD on read; on unencrypted volumes a
  modified map could retarget chunks -- threat-model decision (seal the
  checkpoints vs accept CRC-only).
- VB-D2 `parseFlatSection` (`snapshot.go` ~548-551) sizes a map from a
  uint32 before per-record bounds checks -- validate
  `numBlocks*recordSize <= remaining` before allocating.

### Viperblock -- confirmed-safe
- Deterministic 12-byte GCM nonce with domain separation (chunk/WAL/meta),
  `MaxSeqNum` refusal, and sequence high-water reservation on encrypted
  open -- no obvious nonce-reuse path.
- Chunk/WAL/meta reads verify the auth tag; tamper and cross-volume splice
  are covered by encryption tests.
- Runtime-vs-persisted encryption mismatch, missing key, and key-
  fingerprint mismatch all fail closed before mutating state; pre-
  encryption magics rejected under an encrypted runtime.
- Master key load rejects group/other-readable key files; raw key bytes
  not retained.
- Encrypted WAL recovery uses fixed record size + `io.ReadFull` and is
  fail-closed on integrity failure; successful WAL files deleted only
  after the recovered state is persisted.
- Read/write bounds-check against volume size; pending write memory is
  bounded by backpressure.

---

## Predastore (S3-compatible object store)

SigV4 header verification, IAM default-deny, and fragment encryption are
carefully done. The highest-priority items are inter-node transport
authentication and streaming-upload integrity/limits.

### PD-1 -- QUIC shard transport does not authenticate peers  [CRITICAL]
- Where: `quic/quicserver/server.go` ~449-459 -- server `tls.Config` sets
  its own cert and TLS 1.3 but no `ClientAuth`.
- Concern: the shard listener proves its own identity but does not require
  a client certificate, so any party that can reach the port and speak the
  protocol could read/write/delete shards. This is the storage data plane.
- Direction: require mutual TLS (`RequireAndVerifyClientCert`) against a
  cluster CA; bind node identity to the certificate and reject unknown
  peers. Confirm the deployment exposes these ports only on a trusted
  network in the meantime.

### PD-2 -- Raft consensus transport does not authenticate peers  [CRITICAL]
- Where: `s3db/raft_streamlayer.go` ~37-56 -- encrypted, client verifies
  the server cert, but the listener sets no `ClientAuth`.
- Concern: a reachable Raft port can accept unauthenticated TLS dialers
  into the consensus plane.
- Direction: mutual TLS on both dial and accept; pin peer certs to the
  configured node set.

### PD-3 -- Streaming (aws-chunked) per-chunk signatures are not verified  [HIGH]
- Where: `s3/chunked/chunked.go` ~163-166 -- chunk extensions after `;`
  (which carry `chunk-signature=...`) are stripped and discarded.
- Concern: `STREAMING-AWS4-HMAC-SHA256-PAYLOAD` uploads pass the header
  signature but the per-chunk signatures that bind the body are not
  checked, so streamed body integrity is not cryptographically verified.
- Direction: implement streaming SigV4 (seed signature + per-chunk HMAC
  chain), or reject streaming mode until it is implemented.

### PD-4 -- Trailer checksum is computed but never verified on the write path  [HIGH]
- Where: `backend/distributed/put.go` ~64-67; same pattern in
  `.../multipart.go` ~142-144 -- decoder is created but
  `VerifyTrailerChecksum()` is never called.
- Concern: a corrupted or substituted chunked body can be stored even when
  a trailer checksum was advertised.
- Direction: call the trailer verification after decode EOF when trailers/
  checksum mode is advertised; fail closed on mismatch or missing trailer.

### PD-5 -- UploadPart buffers the whole part before the size check  [HIGH]
- Where: `backend/multipart/multipart.go` ~183-185 and
  `backend/distributed/multipart.go` ~146-158 -- `io.ReadAll(TeeReader)`
  then a later `partSize > MaxPartSize` check.
- Concern: a single UploadPart can drive a multi-GiB in-memory allocation
  before the 5 GiB cap is enforced (memory-exhaustion risk).
- Direction: stream through `io.LimitReader(r, MaxPartSize+1)` (or a temp
  file) and reject before materializing the full buffer.

### PD-6 -- Single-part PutObject has no maximum object size  [HIGH]
- Where: `backend/distributed/put.go` ~61-67 -- `io.Copy(tmpFile, reader)`
  with no size cap (a cap exists only at multipart-complete).
- Concern: a single PutObject can fill temp disk / store capacity without
  an API-level bound.
- Direction: enforce Content-Length / decoded length against a configured
  maximum; wrap the body with a limit reader and map overflow to
  `EntityTooLarge`.

### PD-7 -- Body hash (`x-amz-content-sha256`) not compared to the body  [MEDIUM]
- Where: `pkg/sigv4/parse.go` ~231-245 -- the signed digest header value is
  used verbatim and not compared to a server-computed body hash.
- Concern: for a hex-digest content hash, a client could sign one digest
  and send a different body; integrity then rests on transport + client
  honesty. AWS verifies the digest outside unsigned/streaming modes.
- Direction: when the header is a hex digest, stream-hash the body (bounded
  by Content-Length) and reject on mismatch; keep UNSIGNED-PAYLOAD and
  streaming as explicit, documented weaker modes.

### PD-8 -- Range header parse errors are ignored  [MEDIUM]
- Where: `s3/httpserver.go` ~576-588 -- `start, _ := strconv.ParseInt(...)`
  (errors discarded); combined with a multi-shard fallback that can
  reconstruct the whole object (`get.go` ~179-195).
- Concern: a malformed Range can silently default and, on large objects,
  trigger expensive full-object reconstruction.
- Direction: reject unparsable/negative ranges with `InvalidRange`; do not
  fall back to full reconstruction for a small requested range on a huge
  object without a cap.

### PD-9 -- Authenticated s3db PUT reads an unbounded body  [MEDIUM]
- Where: `s3db/server.go` ~298 -- `io.ReadAll(r.Body)`.
- Direction: `http.MaxBytesReader` with a hard cap before `ReadAll`.

### PD-10 -- s3db scan `limit` has no upper bound  [MEDIUM]
- Where: `s3db/server.go` ~393-397 -- client `limit` overrides the default
  with no clamp.
- Direction: clamp to a maximum; reject non-positive values.

### PD-11 -- Unauthenticated `/status` discloses cluster topology  [LOW]
- Where: `s3db/server.go` ~139-141, 211-223 -- `/health` and `/status`
  bypass auth; `/status` returns leader/term/commit-index details.
- Direction: keep `/health` minimal; require auth (or a separate admin
  listener / network ACL) for `/status`.

### PD-12 -- Rate limiting is opt-in  [LOW]
- Where: `s3/httpserver.go` ~52-53; `ratelimit/config.go` ~4-5.
- Direction: default-enable conservative per-account limits in production
  configs; document required settings.

### PD-13 -- Object-key validation helper is unused on the request path  [LOW]
- Where: `s3/validation.go` ~15-22 (`IsValidKeyName` only checks UTF-8; no
  request-path callers). Mitigated because storage keys are SHA-256 hashes,
  not filesystem paths.
- Direction: enforce max key length (1024) and reject empty/NUL keys from
  the Put/Get/Delete/Multipart handlers.

### PD-14 / PD-15 -- Dead anonymous-access config flags; bucket-policy TODO  [INFO]
- `s3/s3.go` ~79-80: `AllowAnonymousListing`/`AllowAnonymousAccess` appear
  only as config fields with no runtime reads (actual control is per-bucket
  `Public`); wire or remove them and document the real switch.
- `s3/policy.go` ~29: resource/bucket policies for cross-account grants are
  a TODO -- current behavior is default-deny for cross-account, which is
  safe; implement before broadening cross-account access.

### Predastore -- deeper review
- PD-D1 32-bit store id under a shared cluster master key
  (`store/state.go` ~34-38): track collision headroom for large fleets /
  cloned volumes; consider a wider id or per-node key derivation.
- PD-D2 CompleteMultipart loads all part bodies into memory in parallel
  (`multipart.go` ~337-370) -- memory amplification for large objects.
- PD-D3 confirm TOML `Public` and backend-metadata ownership stay aligned
  in production ops.

### Predastore -- confirmed-safe
- SigV4 signature compare via `hmac.Equal`; clock skew 15 min; presign max
  age 7 days; future/expired rejected.
- s3db refuses to start with empty credentials (fail-closed).
- IAM default-deny (deny wins / implicit deny); empty policy denies.
- Public buckets allow GET/HEAD only.
- TLS 1.3 minimum on S3 HTTP, s3db HTTP, Raft, and QUIC; no
  `InsecureSkipVerify` on production dialers (only tests/probe).
- Fragment encryption: GCM Open with AAD; nonce `fragNum||storeID`; high-
  water reservation to avoid nonce reuse; master key `0600` fail-closed.
- Object identity is a content hash -> segment files are numeric, not key-
  derived paths (path traversal into storage is not reachable via keys).
- Multipart part numbers validated 1..10000 before store.

---

## Cross-cutting themes

1. Condition-aware authorization is missing in the shared IAM policy type
   (SP-2, SP-3). Because spinifex imports predastore's `iampolicy`, fixing
   the type benefits both. This is the single highest-leverage correctness
   fix for least-privilege intent.
2. Inter-node transports authenticate the server but not the client
   (PD-1, PD-2). Mutual TLS across QUIC + Raft closes the storage/consensus
   planes; until then those ports must live on a trusted network only.
3. Request-controlled sizes reach allocations before limits in several
   places (SP-6, VB-3, VB-5, PD-5, PD-6, PD-9, PD-10). A consistent
   "limit-before-allocate" pass across the request surfaces would retire a
   whole class of memory-exhaustion risk.
4. Secrets on argv / in logs (SP-5, VB-4). A redaction pass on config
   logging plus moving secrets off command lines is cheap and high value.
5. One durability defect (VB-1) is not strictly "security" but is silent
   data loss; it belongs at the top of the queue with the criticals.

---

## Remediation priority (suggested order for the deeper-dive agent)

Confirm-then-fix each; write the repro before the fix.

1. OPS-1  rotate the working-tree token (minutes; do first).
2. VB-1   Close-after-failed-drain data loss (CRITICAL, durability).
3. PD-1 + PD-2  mutual TLS on QUIC + Raft (CRITICAL, or document the
         trusted-network requirement explicitly as an interim).
4. SP-4   close the authorization fail-open branches.
5. SP-1   gate STS on the caller identity policy.
6. SP-2 + SP-3  condition-aware policy (or fail-closed reject) in the
         shared iampolicy type.
7. PD-3 + PD-4  streaming per-chunk signature + trailer checksum
         verification.
8. PD-5 + PD-6 + SP-6 + VB-3  limit-before-allocate on the big request
         surfaces.
9. SP-5 + VB-4  secrets off argv / out of logs.
10. VB-2  volume-name sanitization at the engine boundary.
11. Remaining MEDIUM/LOW items (SP-7..SP-11, VB-5..VB-11, PD-7..PD-13)
    grouped by file to minimize churn.
12. Deeper-review items (SP-D*, VB-D*, PD-D*): investigate and either
    downgrade with evidence or promote to a fix.

Every item above is provisional pending runtime confirmation. None has
been reproduced or fixed in this pass.
