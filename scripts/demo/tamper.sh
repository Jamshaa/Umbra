#!/usr/bin/env bash
# tamper.sh: prove the demo receipt chain detects manipulation.
# Copies the chain, flips one byte in a middle receipt, and verifies:
# the copy must fail naming the position; the original must pass.
# Requires UMBRA_RECEIPT_HMAC_KEY (same ephemeral key as the run).
set -euo pipefail
source "$(dirname "$0")/../lib/guard.sh"

cd "$(dirname "$0")/../.."
CHAIN="${1:-.demo/receipts.chain}"
[ -f "$CHAIN" ] || { echo "tamper: no chain at $CHAIN (run the demo first)" >&2; exit 1; }
[ -n "${UMBRA_RECEIPT_HMAC_KEY:-}" ] || { echo "tamper: UMBRA_RECEIPT_HMAC_KEY is not set (see SECURITY.md)" >&2; exit 1; }

TMP="$(mktemp)"
trap 'rm -f "$TMP"' EXIT
cp "$CHAIN" "$TMP"
# Flip exactly one byte inside the second line's payload.
sed -i '2s/./X/20' "$TMP"

set +e
OUT="$(./.build/host/umbra receipts verify "$TMP" 2>&1)"
CODE=$?
set -e
if [ "$CODE" -eq 0 ]; then
	echo "tamper: FAILED (tampered chain verified)"
	exit 1
fi
echo "tamper detected: $OUT"
./.build/host/umbra receipts verify "$CHAIN"
echo "original chain intact"
