#!/usr/bin/env bash
set -euo pipefail

section() {
  printf '\n== %s ==\n' "$1"
}

run_optional() {
  local label="$1"
  shift
  section "$label"
  if command -v "$1" >/dev/null 2>&1; then
    "$@" || true
  else
    printf '%s: not found\n' "$1"
  fi
}

section "recommended environment"
printf 'KYLINGUARD_HOME=%s\n' "${KYLINGUARD_HOME:-/opt/kylin-guard-agent}"
printf 'TRACESHIELD_CORE_PATH=%s\n' "${TRACESHIELD_CORE_PATH:-/opt/traceshield-core}"
printf 'AUDIT_CORE_URL=%s\n' "${AUDIT_CORE_URL:-http://127.0.0.1:8001}"
printf 'AGENT_GO_PORT=%s\n' "${AGENT_GO_PORT:-8080}"
printf 'AUDIT_CORE_PORT=%s\n' "${AUDIT_CORE_PORT:-8001}"
printf 'EINO_ENABLED=%s\n' "${EINO_ENABLED:-false}"

section "current user"
id || true
printf 'whoami: %s\n' "$(whoami 2>/dev/null || printf unknown)"

section "current working directory"
pwd

section "uname -m"
arch="$(uname -m || true)"
printf '%s\n' "$arch"
case "$arch" in
  x86_64)
    printf 'architecture: x86_64, suitable for Kylin VM precheck\n'
    ;;
  loongarch64)
    printf 'architecture: loongarch64, target LoongArch validation environment\n'
    ;;
  aarch64)
    printf 'architecture: aarch64, ARM64 compatibility precheck\n'
    ;;
  *)
    printf 'architecture: %s, unclassified; verify toolchain support manually\n' "$arch"
    ;;
esac

section "/etc/os-release"
if [[ -r /etc/os-release ]]; then
  cat /etc/os-release
else
  printf '/etc/os-release: not readable\n'
fi

run_optional "go version" go version
run_optional "python3 --version" python3 --version
run_optional "pip3 --version" pip3 --version
run_optional "gcc --version" gcc --version
run_optional "systemctl --version" systemctl --version
run_optional "journalctl --version" journalctl --version
run_optional "lsof -v" lsof -v

section "network inspection tool"
if command -v ss >/dev/null 2>&1; then
  ss --version || true
elif command -v netstat >/dev/null 2>&1; then
  printf 'netstat: available at %s\n' "$(command -v netstat)"
else
  printf 'ss/netstat: not found\n'
fi
