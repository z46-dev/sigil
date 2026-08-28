# Sigil

![CI](https://img.shields.io/github/actions/workflow/status/z46-dev/sigil/ci.yml?branch=main&event=push&label=CI)
![Made with Go](https://img.shields.io/badge/-Made_with_Go-007d9c?logo=go&logoColor=white)

Sigil is an experimental, self-hosted browser fingerprinting service written in Go targeting WebAssembly. It collects permissionless browser signals, creates snapshot IDs, and uses server-side history to assign durable visitor IDs.

Sigil is a research-quality project, not a production fraud-prevention platform. Its current single-machine Playwright baseline achieved 66.7% accuracy with a 40% false-new rate. The API has strict validation, rate limits, same-origin checks, and single-use challenges, but browser signals remain attacker-controlled. Authentication, automatic retention, and server-observed network signals are not implemented.

Sigil is worth more (and was designed) for research and education than for production use, but I am hoping to improve it over time and get it to a point where it can be used in production. If you are interested in contributing, feel free to open an issue or a pull request!

## Comparison

Legend: ✅ fully supported · ⚠️ partial or experimental · ❌ not supported

| Capability | Sigil | [FingerprintJS OSS](https://github.com/fingerprintjs/fingerprintjs) | [ThumbmarkJS OSS](https://github.com/thumbmarkjs/thumbmarkjs) | [Fingerprint Identification](https://fingerprint.com/) | [ThumbmarkJS API](https://www.thumbmarkjs.com/) |
| --- | :---: | :---: | :---: | :---: | :---: |
| Open source | ✅ | ✅ | ✅ | ❌ | ❌ |
| Fully self-hosted | ✅ | ✅ | ✅ | ❌ | ❌ |
| Browser fingerprint | ✅ | ✅ | ✅ | ✅ | ✅ |
| Server-side visitor history | ✅ | ❌ | ❌ | ✅ | ✅ |
| Cross-browser matching | ⚠️ | ❌ | ❌ | ❌ | ❌ |
| Server/network signals | ❌ | ❌ | ❌ | ✅ | ✅ |
| Bot and fraud signals | ❌ | ❌ | ⚠️ | ✅ | ✅ |
| Native mobile SDKs | ❌ | ❌ | ❌ | ✅ | ❌ |
| Tamper and replay protection | ⚠️ | ❌ | ❌ | ✅ | ⚠️ |
| Production-calibrated accuracy | ❌ | ⚠️ | ⚠️ | ✅ | ✅ |

The commercial columns describe hosted products, not guarantees for every browser or deployment. Fingerprint's own documentation describes web visitor IDs as browser-specific. ThumbmarkJS reports roughly 80% uniqueness for its open-source client and over 99% uniqueness for its hosted API; uniqueness is not the same as longitudinal identification accuracy.

## Run it

Requirements: Go 1.26+, Node.js 22+, and npm.

```bash
go mod download
npm ci
bash ./scripts/build.sh
go run ./src
```

Windows PowerShell:

```powershell
go mod download
npm ci
./scripts/build.ps1
go run ./src
```

The server listens on `127.0.0.1:8080` by default. Configuration is stored in `config.toml`.

## Verify

```bash
go test ./...
go vet ./...
npm run evaluate
```

The evaluation requires Playwright browsers:

```bash
npx playwright install chromium firefox webkit
```

## Privacy

Browser fingerprints may be personal data. Deploy Sigil only with an appropriate lawful purpose, disclosure or consent, access controls, retention limits, and a deletion process. Never use its visitor ID as sole proof of identity, authorization, or fraud.

## License

[GNU AGPLv3](LICENSE)
