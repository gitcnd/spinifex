# ENVIRONMENT -- Spinifex Outposts-parity project (machine audit + traps)

<!-- Produced by probing on 2026-07-27, not by assumption. Update the same
     day a trap bites. -->

## Host machine

- OS: Oracle Linux Server 9.3 (RHEL family, kernel 5.15.0-102.110.5.1.el9uek).
  NOTE: Spinifex officially supports Debian 13 (Ubuntu/Debian family). The
  host distro is NOT a supported install target; development cluster should
  run inside KVM guests (see fork F3 in DECISIONS.md).
- CPU: Intel Xeon E5-2697 v4 @ 2.30GHz, 72 hardware threads, VT-x enabled.
- Nested virtualization: ENABLED (/sys/module/kvm_intel/parameters/nested = Y).
  EC2-style guests inside our Debian dev VMs are possible.
- /dev/kvm: present, world read/write (crw-rw-rw-), usable without root.
- RAM: 188 GiB total; at audit time 151 GiB in use, ~37 GiB available,
  swap 160 GiB (6.9 GiB in use). TRAP: the machine is heavily loaded by
  other work; budget VM memory conservatively and confirm with the human.
- Disks:
  - / : 450 GB SSD (sdg3), 256 GB free.
  - /home : 27 TB pool ("zcdcpool"), 6.7 TB free. Workspace lives here.
  - nvme0n1 / nvme1n1: 2x 1.9 TB Samsung NVMe, both marked isw_raid_member
    (Intel software RAID). /proc/mdstat empty at audit -- array not
    assembled. DO NOT touch without explicit human permission; ideal
    candidates for Viperblock WAL testing if the human frees them.
- GPU: not probed yet (not needed for storage/control-plane work).

## Toolchains

- Go: system go1.21.13 (below the repo's go 1.26.5 requirement).
  VERIFIED FIX: run all go commands with GOTOOLCHAIN=auto; it downloads and
  uses go1.26.5 automatically (verified 2026-07-27:
  "GOTOOLCHAIN=auto go version" -> go1.26.5;
  "GOTOOLCHAIN=auto go build ./spinifex/gateway/" -> success).
- QEMU: RHEL packaging, qemu-kvm 7.2.0-15.el9 (qemu-img confirmed on PATH).
  qemu-system-x86_64 NOT confirmed on PATH; RHEL puts the binary at
  /usr/libexec/qemu-kvm. Debian guests will carry their own QEMU for the
  actual Spinifex compute plane.
- Containers: podman 4.9.4 emulating the docker CLI.
- libvirt: installed but inactive; virsh NOT on PATH.
- nbdkit: NOT installed on host (Viperblock needs it -- inside guests).
- OVN/OVS: NOT installed on host (Spinifex VPC needs it -- inside guests).
- sudo: requires a password. Anything needing root is a NEEDS HUMAN item.

## Repos and sources

- Workspace: /home/devnull/Downloads/github/spinifex (git, branch main,
  clean except untracked editor folders; latest commit 220141b1
  "release: update go.mod for v1.13.0").
- Dependency sources (read-only, from module cache):
  - /home/devnull/go/pkg/mod/github.com/mulgadc/viperblock@v1.13.0
  - /home/devnull/go/pkg/mod/github.com/mulgadc/predastore@v1.13.0
  To MODIFY them we need sibling clones + go.work replace; the repo ships
  scripts/clone-deps.sh for exactly this.

## Git auth

- Remote `origin` = https://github.com/gitcnd/spinifex (the human's fork).
- Fine-grained token in .env (gitignored; NEVER print or commit it).
  Verified 2026-07-27: GET /user -> 200; gitcnd/spinifex push:true,
  admin:true; cannot fork other repos ("Bad credentials" outside scope).
- Push recipe (keeps the token out of .git/config and logs):
    set -a && source .env && set +a
    git -c credential.helper='!f() { test "$1" = get && printf \
      "username=%s\npassword=%s\n" "$GITHUB_USERNAME" "$GITHUB_TOKEN"; }; f' \
      push -u origin <branch>
- gh CLI is NOT installed; use curl + $GITHUB_TOKEN for API calls.

## Traps

- GOTOOLCHAIN: plain "go build/test" fails with a version error. Symptom:
  "go.mod requires go >= 1.26.5". Fix: prefix with GOTOOLCHAIN=auto (or
  install Go 1.26.5+ and put it first in PATH).
- Shell prints "dump_bash_state: command not found" after every command in
  this harness; harmless noise, ignore it.
- Module cache is read-only; editing viperblock/predastore there will fail
  or corrupt the cache. Always use sibling clones + go.work.
- Host distro mismatch: install scripts (install.mulgadc.com, setup-ovn.sh)
  assume Debian/Ubuntu and will not run on Oracle Linux 9. Run them inside
  Debian 13 guests only.
- Viperblock v1.13.0 module zip is missing the nbd/ directory (nbdkit
  plugin source) although the Makefile references it -- the plugin lives in
  the git repo, not the published module. Clone the repo when building NBD.
