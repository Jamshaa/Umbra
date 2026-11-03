#!/usr/bin/env bash
# reset.sh: clean demo stage between takes. Never touches bench/results
# or the repository. Idempotent.
set -euo pipefail
source "$(dirname "$0")/../lib/guard.sh"

cd "$(dirname "$0")/../.."

# Demo namespace contents (keep the namespace itself).
kubectl --context kind-umbra -n demo delete deploy,svc,cm,secret,sa,role,rolebinding,cronjob,job,pod,pdb,hpa --all --ignore-not-found >/dev/null 2>&1 || true
# Sandbox contents (keep namespace and its baseline policy/quota).
kubectl --context kind-umbra -n umbra-sandbox delete deploy,svc,cm,secret,sa,role,rolebinding,cronjob,job,pod --all --ignore-not-found >/dev/null 2>&1 || true
# Published findings from scan/diagnose runs.
kubectl --context kind-umbra delete findings -l umbra.io/source --ignore-not-found >/dev/null 2>&1 || true
rm -f .demo/receipts.chain .demo/audit.log
echo "reset: demo stage clean"
