#!/usr/bin/env python3
"""Claims lint: docs must not present unbuilt designs as existing facts.

Stdlib only. Scans the public docs and fails when a line claims a
feature that does not exist, unless the same line carries an explicit
qualifier (roadmap / not built / planned / design only / no existe...).

Usage: python3 scripts/docs/claims_lint.py   (exit 1 on any violation)
"""
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
FILES = [
    ROOT / "README.md",
    ROOT / "docs" / "DEMO.md",
    ROOT / "docs" / "META.md",
    ROOT / "docs" / "ARCHITECTURE.md",
    ROOT / "docs" / "STATUS.md",
    ROOT / "docs" / "AI_USE.md",  # scanned once it exists (created in A6)
]
QUALIFIERS = [
    "roadmap",
    "no construido",
    "not built",
    "diseño",
    "design only",
    "planned",
    "no existe",
    "doesn't exist",
    "does not exist",
]
GROUPS = [
    ("shadow mode", ["modo sombra", "shadow mode"]),
    ("kubectl plugin", ["plugin de kubectl", "kubectl plugin", "krew"]),
    ("restart recovery", ["recuperación tras reinicio", "restart recovery"]),
    ("attestation", ["attestation", "atestación", "non-repudiation", "no repudio"]),
]


def violations(path):
    out = []
    for i, line in enumerate(path.read_text(encoding="utf-8").splitlines(), 1):
        low = line.lower()
        if "<confirm-by-author>" in low:
            out.append((i, "unresolved <CONFIRM-BY-AUTHOR> marker", line.strip()))
            continue
        qualified = any(q in low for q in QUALIFIERS)
        for label, phrases in GROUPS:
            if not qualified and any(p in low for p in phrases):
                # Honest negations ("no es una firma ni ofrece no
                # repudio") state the opposite of the banned claim.
                if label == "attestation" and re.search(r"\b(no|not|never|ni)\b", low):
                    continue
                out.append((i, label, line.strip()))
        # Helm presented as existing.
        if not qualified and ("helm chart" in low or "helm install" in low):
            out.append((i, "helm as existing", line.strip()))
        # Automatic PR / auto-apply: allowed only in negative sentences.
        if any(p in low for p in ["pr automático", "automatic pr", "auto-apply", "auto apply"]):
            if not re.search(r"\b(no|not|never|jamás)\b", low):
                out.append((i, "auto-apply/automatic PR claimed", line.strip()))
        # Receipt signatures: HMAC is tamper-evidence, not a signature.
        if ("signature" in low or "firma" in low) and any(
            w in low for w in ["receipt", "recibo", "recibos"]
        ):
            if "not a" not in low and "no es" not in low:
                out.append((i, "receipt signature claimed", line.strip()))
    return out


def main():
    bad = []
    for f in FILES:
        if not f.exists():
            continue
        for line_no, label, text in violations(f):
            bad.append(f"{f.relative_to(ROOT)}:{line_no}: [{label}] {text}")
    if bad:
        for b in bad:
            print("claims_lint:", b, file=sys.stderr)
        return 1
    print("claims lint clean")
    return 0


if __name__ == "__main__":
    sys.exit(main())
