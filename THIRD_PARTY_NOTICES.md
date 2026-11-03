# Third-party notices

This project is Apache-2.0 (see `LICENSE`). Direct third-party components
used at runtime are listed below; font license texts ship verbatim in
`ui/src/styles/fonts/LICENSES/`.

## Fonts (SIL Open Font License 1.1)

| Font | Copyright holder (per its OFL) | License | Source |
|---|---|---|---|
| Press Start 2P | Copyright 2012 The Press Start 2P Project Authors (cody@zone38.net), with Reserved Font Name "Press Start 2P" | SIL Open Font License 1.1 (`ui/src/styles/fonts/LICENSES/press-start-2p-OFL.txt`) | https://github.com/google/fonts/tree/main/ofl/pressstart2p |
| VT323 | Copyright 2011, The VT323 Project Authors (peter.hull@oikoi.com) | SIL Open Font License 1.1 (`ui/src/styles/fonts/LICENSES/vt323-OFL.txt`) | https://github.com/google/fonts/tree/main/ofl/vt323 |
| DM Mono (400, 500) | Copyright 2020 The DM Mono Project Authors | SIL Open Font License 1.1 (`ui/src/styles/fonts/LICENSES/dm-mono-OFL.txt`) | https://github.com/googlefonts/dm-mono |

Font binaries are self-hosted (`ui/src/styles/fonts/*.woff2`, latin subset);
the UI makes no external font requests.

## Go direct dependencies (from go.mod)

| Module | Version | License |
|---|---|---|
| k8s.io/api | v0.37.0 | Apache-2.0 |
| k8s.io/apimachinery | v0.37.0 | Apache-2.0 |
| k8s.io/client-go | v0.37.0 | Apache-2.0 |
| sigs.k8s.io/controller-runtime | v0.25.2 | Apache-2.0 |
| sigs.k8s.io/yaml | v1.6.0 | MIT |

## npm direct production dependencies (from ui/package.json)

| Package | Version (lockfile) | License |
|---|---|---|
| react | 19.3.1 | MIT |
| react-dom | 19.3.1 | MIT |
