#!/usr/bin/env bash
# leakcheck.sh: fail if any to-be-shown text leaks secrets or private
# context. Default input: the newest final rehearsal log; explicit
# files may be passed. Reports rule + line number only, never the match.
set -euo pipefail
cd "$(dirname "$0")/../.."

if [ "$#" -gt 0 ]; then
	FILES=("$@")
else
	# shellcheck disable=SC2207
	FILES=($(ls -t .demo/rehearsal-*.log 2>/dev/null | head -1 || true))
	[ "${#FILES[@]}" -gt 0 ] || {
		echo "leakcheck: no .demo/rehearsal-*.log (run the final pass first) and no files given" >&2
		exit 1
	}
fi

fail=0
check() { # $1=label $2=pattern
	local f
	for f in "${FILES[@]}"; do
		while IFS= read -r n; do
			echo "leakcheck: $1 in $f:$n" >&2
			fail=1
		done < <(grep -n -E "$2" "$f" 2>/dev/null | cut -d: -f1 || true)
	done
}
check "clave/token (gitleaks/sanitizer shapes)" '(AKIA|ASIA)[0-9A-Z]{16}|(ghp|gho|ghu|ghs|ghr)_[A-Za-z0-9]{20,}|github_pat_[A-Za-z0-9_]{20,}|xox[bpars]-[A-Za-z0-9-]{10,}|\bsk-[A-Za-z0-9_-]{20,}|eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{8,}|Bearer[[:space:]]+[A-Za-z0-9._~+/=-]{10,}|-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----'
[ -n "${HOME:-}" ] && check '$HOME literal' "$(printf '%s' "$HOME" | sed 's/[][\.*^$/]/\\&/g')"
[ -n "${USER:-}" ] && check '$USER literal' "(^|[^A-Za-z0-9])$(printf '%s' "$USER" | sed 's/[][\.*^$/]/\\&/g')([^A-Za-z0-9]|$)"
check 'hostname literal' "$(hostname | sed 's/[][\.*^$/]/\\&/g')"
check 'ruta Windows de usuario' '[A-Za-z]:\\Users\\'
# Emails other than example.com / noreply.
for f in "${FILES[@]}"; do
	while IFS= read -r n; do
		echo "leakcheck: correo no example.com/noreply in $f:$n" >&2
		fail=1
	done < <(grep -n -E '[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+' "$f" 2>/dev/null | grep -v -E '@example\.com|noreply' | cut -d: -f1 || true)
done
check 'IP privada real' '\b(10\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}|172\.(1[6-9]|2[0-9]|3[01])\.[0-9]{1,3}\.[0-9]{1,3}|192\.168\.[0-9]{1,3}\.[0-9]{1,3})\b'

if [ "$fail" != 0 ]; then
	echo "leakcheck: FAIL" >&2
	exit 1
fi
echo "leakcheck: OK (${FILES[*]})"
