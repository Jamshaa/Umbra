#!/usr/bin/env python3
"""check.py: coherence gate for the bench catalogs (the scoreboard catalog).

Every case in bench/cases*.yaml must be internally consistent and
resolvable from the repo: ids unique across all files, known
category/status, injector present unless planned, verifier resolvable
(Go symbol in internal/verify or scripts/verify/*.sh), and no orphan
injector/demo file without a case. Anything else means a catalog
drifted from the tree. Exit 0 when coherent, 1 otherwise.

Local-only: no cluster, no network.
"""

import glob
import os
import re
import sys

import yaml

HERE = os.path.abspath(__file__)
ROOT = os.path.dirname(os.path.dirname(os.path.dirname(HERE)))
CASES_GLOB = os.path.join(ROOT, "bench", "cases*.yaml")
VERIFY_GO = os.path.join(ROOT, "internal", "verify", "verify.go")

CATEGORIES = {"Resources", "Reliability", "Security", "RBAC", "Config", "Trap"}
STATUSES = {"planned", "injector-ready", "verified"}


def fail(errors, msg):
    errors.append(msg)


def main():
    errors = []
    catalog_files = sorted(glob.glob(CASES_GLOB))
    if not catalog_files:
        print("no bench catalogs found")
        return 1
    catalogs = []
    for path in catalog_files:
        with open(path, encoding="utf-8") as fh:
            catalogs.append((os.path.basename(path), yaml.safe_load(fh)))

    verify_src = open(VERIFY_GO, encoding="utf-8").read()
    go_symbols = set(re.findall(r"func (Verify\w+)\(", verify_src))

    seen_ids = set()
    cased_injectors = set()
    total = 0
    ready = 0
    for fname, catalog in catalogs:
        for case in catalog.get("cases", []):
            cid = case.get("id", "?")
            total += 1
            if cid in seen_ids:
                fail(errors, f"duplicate id across catalogs: {cid}")
            seen_ids.add(cid)
            if case.get("category") not in CATEGORIES:
                fail(errors, f"{cid}: unknown category {case.get('category')!r}")
            if case.get("status") not in STATUSES:
                fail(errors, f"{cid}: unknown status {case.get('status')!r}")
            if not isinstance(case.get("trap"), bool):
                fail(errors, f"{cid}: trap flag must be boolean")
            if case.get("category") == "Trap" and not case.get("trap"):
                fail(errors, f"{cid}: Trap category without trap=true")
            if case.get("category") != "Trap" and case.get("trap"):
                fail(errors, f"{cid}: non-Trap category with trap=true")
            if case.get("status") != "planned":
                ready += 1

            inj = case.get("injector", "")
            if case.get("status") == "planned":
                if inj and os.path.exists(os.path.join(ROOT, inj)):
                    fail(errors, f"{cid}: planned but injector exists ({inj})")
                continue
            cased_injectors.add(inj)
            if not inj or not os.path.exists(os.path.join(ROOT, inj)):
                fail(errors, f"{cid}: missing injector {inj!r}")
                continue

            ver = case.get("verifier", "")
            if ver.startswith("internal/verify."):
                sym = ver.split(".", 1)[1]
                if sym not in go_symbols:
                    fail(errors, f"{cid}: unknown verifier symbol {ver!r}")
            elif ver.startswith("scripts/"):
                if not os.path.exists(os.path.join(ROOT, ver)):
                    fail(errors, f"{cid}: missing verifier script {ver!r}")
            else:
                fail(
                    errors,
                    f"{cid}: verifier {ver!r} is neither internal/verify.* nor scripts/*",
                )

    for path in sorted(glob.glob("scripts/injectors/*.sh", root_dir=ROOT)):
        path = path.replace(os.sep, "/")
        if path not in cased_injectors:
            fail(errors, f"orphan injector without case: {path}")
    demo_listed = set()
    with open(os.path.join(ROOT, "demo", "kustomization.yaml"), encoding="utf-8") as fh:
        for doc in yaml.safe_load_all(fh):
            if isinstance(doc, dict):
                demo_listed.update(doc.get("resources", []) or [])
    for path in sorted(glob.glob("demo/f*.yaml", root_dir=ROOT)):
        if os.path.basename(path) not in demo_listed:
            fail(errors, f"demo manifest not in kustomization: {path}")
    trap_refs = set()
    for path in sorted(glob.glob("scripts/traps/*.sh", root_dir=ROOT)):
        with open(os.path.join(ROOT, path), encoding="utf-8") as fh:
            trap_refs.update(re.findall(r"traps/[^\s\"']+", fh.read()))
    for path in sorted(glob.glob("traps/*", root_dir=ROOT)):
        rel = path.replace(os.sep, "/")
        if rel not in trap_refs and os.path.basename(path) not in {
            os.path.basename(r) for r in trap_refs
        }:
            fail(errors, f"orphan trap fixture, unreferenced by any trap script: {rel}")

    if errors:
        print("\n".join(errors))
        return 1
    print(f"ok: {total} cases coherent, {ready} injector-ready+")
    return 0


if __name__ == "__main__":
    sys.exit(main())
