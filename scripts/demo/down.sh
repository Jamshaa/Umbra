#!/usr/bin/env bash
# down.sh: removes the demo namespace entirely (with all its contents).
# Touches no other namespace. Idempotent: re-running is safe.
# Only external binary: kubectl (in PATH).
set -eu
source "$(dirname "$0")/../lib/guard.sh"

NAMESPACE="demo"

kubectl --context kind-umbra delete namespace "$NAMESPACE" --ignore-not-found=true
