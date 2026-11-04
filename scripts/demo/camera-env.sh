#!/usr/bin/env bash
# camera-env.sh: SOURCE this in the demo terminal (do not execute it).
# Short prompt, no history, secrets read silently, nothing env printed.
#   source scripts/demo/camera-env.sh
if [ "${BASH_SOURCE[0]:-}" = "$0" ]; then
	echo "camera-env: source me, do not execute: source scripts/demo/camera-env.sh" >&2
	exit 2
fi
export PS1='umbra$ '
export HISTFILE=/dev/null
export HISTSIZE=0
export HISTFILESIZE=0
set +o history 2>/dev/null || true
if [ -z "${MODEL_API_KEY:-}" ]; then
	read -rsp 'MODEL_API_KEY: ' MODEL_API_KEY
	echo
	export MODEL_API_KEY
fi
if [ -z "${UMBRA_RECEIPT_HMAC_KEY:-}" ]; then
	read -rsp 'UMBRA_RECEIPT_HMAC_KEY: ' UMBRA_RECEIPT_HMAC_KEY
	echo
	export UMBRA_RECEIPT_HMAC_KEY
fi
echo "camera-env: ready (prompt corto, sin historial, claves leidas con read -rs)"
