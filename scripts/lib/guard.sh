#!/usr/bin/env bash
# guard.sh: abort unless kubectl talks to kind-umbra.
# Source from every script that touches the cluster:
#   source "$(dirname "$0")/../lib/guard.sh"
# Combined with --context kind-umbra on each call this makes it
# impossible to leak a command into a foreign cluster (see AGENTS.md).
CTX="kind-umbra"
CUR="$(kubectl config current-context 2>/dev/null || echo none)"
[ "$CUR" = "$CTX" ] || { echo "guard: current-context is '$CUR', want '$CTX'" >&2; exit 1; }
