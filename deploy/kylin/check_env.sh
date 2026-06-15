#!/usr/bin/env sh
set -u

section() {
  printf '\n== %s ==\n' "$1"
}

run_optional() {
  label="$1"
  shift
  section "$label"
  if command -v "$1" >/dev/null 2>&1; then
    "$@" || true
  else
    printf '%s: not found\n' "$1"
  fi
}

section "uname -m"
uname -m || true

section "/etc/os-release"
if [ -r /etc/os-release ]; then
  cat /etc/os-release
else
  printf '/etc/os-release: not readable\n'
fi

run_optional "go version" go version
run_optional "python3 --version" python3 --version
run_optional "pip3 --version" pip3 --version
run_optional "gcc --version" gcc --version
run_optional "systemctl --version" systemctl --version
