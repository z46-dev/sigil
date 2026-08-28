![Tests](https://img.shields.io/github/actions/workflow/status/z46-dev/sigil/ci.yml?branch=main&event=push&label=CI)
![Made with Go](https://img.shields.io/badge/-Made_with_Go-007d9c?logo=go&logoColor=white)

# sigil

Sigil is an educational, self-hosted browser identification service written in Go. The browser agent compiles to WebAssembly and uses Go's `syscall/js` bridge for browser APIs. Small embedded JavaScript probes may be used where a browser API cannot be expressed practically through that bridge.

The project separates two concepts that are often conflated:

- A **snapshot ID** is a deterministic hash of one observation. It changes when an input signal changes.
- A **visitor ID** is a server-assigned identity reconciled across many changing snapshots. It requires history, confidence scoring, and collision controls.

Sigil implements both concepts. Durable visitor IDs are deliberately conservative: the server creates or reuses an ID only when the evidence clears documented score, coverage, confidence, and collision-separation thresholds. This is an implemented matching policy, not a claim of independently measured real-world accuracy.

## Project status

Status values are **done**, **active**, **planned**, and **research**. A feature is only marked done when it is implemented and covered by the repository's verification workflow.

| Area | Status | Current capability |
| --- | --- | --- |
| Versioned signal schema | done | Schema v3 with explicit JSON fields and identity scopes |
| Basic browser signals | done | Browser, locale, timezone, screen, hardware hints, touch, storage availability, and automation flag |
| Deterministic snapshot ID | done | SHA-256 with URL-safe encoding and schema prefix |
| Go/WASM public API | done | `window.sigil.collect()` and `window.sigil.schemaVersion` |
| Device/browser scopes | done | Domain-separated device, browser, and combined snapshot IDs |
| Canvas rendering | done | Deterministic drawing hashed locally; rendered pixels are not retained |
| WebGL environment | done | Renderer/vendor plus a hash of the sorted extension set |
| Audio rendering | done | Permissionless offline render hashed locally with a three-second timeout |
| Font detection and metrics | done | Conservative candidate set and generic metrics; names are hashed and discarded |
| Feature behavior | planned | Capability and JavaScript-engine behavior probes |
| Server ingestion | done | Strict JSON, 64 KiB limit, schema validation, and server-side ID recomputation |
| Request authenticity | planned | Nonces, signatures, replay limits, and server-only evidence |
| Historical matching | done | Best observation per visitor with explainable weighted similarity |
| Durable visitor IDs | done | Random server IDs; ambiguous matches are refused rather than merged |
| Confidence and collision checks | done | Score, coverage, runner-up margin, collision flag, and evidence report |
| Bot and tamper evidence | research | Explainable risk signals, separate from identity |
| Longitudinal evaluation harness | done | Labeled Chromium, Firefox, and WebKit replay with JSON metrics |
| Population calibration dataset | planned | Real devices and browser updates over an extended collection period |

## Baseline comparison

This table compares architectural capabilities, not marketing accuracy. It is based on the linked upstream documentation and should be reviewed as those projects change.

| Capability | Sigil | FingerprintJS OSS | ThumbmarkJS OSS | ClientJS |
| --- | --- | --- | --- | --- |
| Self-hostable client collector | yes | yes | yes | yes |
| Raw component output | yes | yes | yes | yes |
| SHA-256 snapshot ID with schema prefix | yes | different hash format | different hash format | no; 32-bit hash |
| Go/WASM implementation | yes | no | no | no |
| Canvas and WebGL probes | yes | yes | yes | partial |
| Audio and font probes | yes | yes | yes | partial |
| First-party server reconciliation | yes | no | no | no |
| Longitudinal visitor matching | yes | no | no | no |
| Published reproducible accuracy study | planned | no | no | no |

Comparison references:

- [FingerprintJS repository](https://github.com/fingerprintjs/fingerprintjs)
- [ThumbmarkJS repository](https://github.com/thumbmarkjs/thumbmarkjs)
- [ClientJS repository](https://github.com/jackspirou/clientjs)

Commercial APIs are not treated as open-source baselines. Both Fingerprint and Thumbmark document that their server-side products add historical reconciliation and server-observed signals beyond their client libraries.

## Commercial comparison

This comparison tracks the paid services most directly related to the current architecture. A **documented** entry means the vendor currently describes the capability in the linked official documentation; it is not an independent validation of accuracy. Availability can depend on product tier and platform.

| Capability | Sigil target | Fingerprint paid service | Thumbmark API |
| --- | --- | --- | --- |
| Browser snapshot components | partial | documented | documented |
| Persistent server visitor ID | implemented | documented | documented |
| Historical/server reconciliation | implemented | documented | documented |
| Server-observed network evidence | planned | documented | HTTP, IP, and TLS documented |
| Confidence or uniqueness score | implemented | confidence documented | uniqueness documented |
| Bot detection | research | documented | documented |
| VPN/proxy intelligence | research | documented | VPN, Tor, and datacenter documented |
| Browser tampering detection | research | documented | not documented in reviewed sources |
| Incognito detection | research | documented | not documented in reviewed sources |
| Protected server-side results | planned | Server API, webhooks, and sealed results | webhooks documented |
| Native mobile identification | out of current scope | Android and iOS documented | not documented in reviewed sources |
| End-to-end self-hosting | intended | hosted service | hosted API |
| Reproducible local evaluation | planned | vendor-operated | vendor-operated |

Commercial references:

- [Fingerprint identification overview](https://docs.fingerprint.com/docs/introduction)
- [Fingerprint Smart Signals](https://docs.fingerprint.com/docs/smart-signals-introduction)
- [Fingerprint sealed results](https://docs.fingerprint.com/docs/sealed-client-results)
- [Fingerprint mobile identification](https://docs.fingerprint.com/docs/mobile-identification)
- [Thumbmark API components](https://docs.thumbmarkjs.com/docs/components/pro/)
- [Thumbmark API result format](https://docs.thumbmarkjs.com/docs/installation/basic-usage/)

## Current browser API

Build the WASM agent, serve `client/public`, and call:

```javascript
const combined = await window.sigil.collect();
const device = await window.sigil.collect({ mode: "device" });
const browser = await window.sigil.collect({ mode: "browser" });

console.log(combined.snapshotId);
console.log(device.snapshotId);
console.log(browser.snapshotId);
```

An example result has this shape:

```json
{
  "schemaVersion": 3,
  "snapshotId": "sg3c_...",
  "scope": "device-and-browser",
  "collectedAt": "2026-08-26T12:00:00Z",
  "browser": {
    "userAgent": "...",
    "languages": ["en-US", "en"],
    "timezone": "America/New_York",
    "screenWidth": 1920,
    "screenHeight": 1080
  }
}
```

`collectedAt` is metadata and is deliberately excluded from the snapshot hash.

### Identity scopes

| Mode | ID prefix | Included evidence | Intended use |
| --- | --- | --- | --- |
| `device` | `sg3d_` | Normalized platform, orientation-independent screen dimensions, hardware hints, GPU identity, and hashed font availability | Cross-browser candidate retrieval |
| `browser` | `sg3b_` | User agent, vendor, locale settings, storage capabilities, canvas, audio, WebGL extensions, and font metrics | Detect changes to a browser environment |
| `device-and-browser` | `sg3c_` | Complete versioned observation | Highest-entropy exact snapshot |

The default mode is `device-and-browser`. `combined` is accepted as an alias.

Device mode deliberately excludes user agent, canvas, audio output, WebGL extensions, and font metrics because those signals commonly differ between browser engines. Screen orientation is normalized, and common platform names are normalized using User-Agent Client Hints where available.

The resulting device ID is a conservative exact-match key, not an assertion that two requests came from the same physical device. Many devices share the same hardware configuration, while privacy-focused browsers may alter or suppress the remaining evidence. Sigil now applies server-side candidate matching, confidence, and collision separation, but it will not claim cross-browser accuracy until the thresholds are calibrated on the longitudinal evaluation described below.

## Server identification API

Send a snapshot returned by `sigil.collect()` to:

```http
POST /api/v1/identify
Content-Type: application/json
```

```javascript
const snapshot = await window.sigil.collect({ mode: "device" });
const response = await fetch("/api/v1/identify", {
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify(snapshot),
});
const identity = await response.json();
```

A successful new or returning identity produces a result like:

```json
{
  "decision": "matched",
  "visitorId": "sv1_...",
  "confidence": 0.96,
  "bestScore": 0.98,
  "runnerUpScore": 0.41,
  "margin": 0.57,
  "evidenceCoverage": 0.92,
  "collision": false,
  "candidateCount": 4,
  "evidence": [
    { "name": "webgl-renderer", "weight": 14, "similarity": 1 }
  ]
}
```

Responses use these decisions:

| Decision | Meaning | Persistence behavior |
| --- | --- | --- |
| `new` | No visitor cleared every matching threshold | Creates a random `sv1_` visitor ID and stores the observation |
| `matched` | One visitor cleared every threshold with sufficient separation | Reuses its visitor ID and stores the observation |
| `ambiguous` | Two strong candidates were separated by less than the collision margin | Returns no visitor ID and stores nothing |

New identities return HTTP `201`; matched and ambiguous results return HTTP `200`. Invalid schemas, unknown fields, oversized metadata, or a snapshot ID that does not match the submitted signals return HTTP `400` or `413` as appropriate.

### Matching model

The matcher compares the incoming snapshot against every stored observation in the same scope, then retains only the best observation for each visitor. This prevents a visitor with a long history from occupying several places in the ranking.

Device evidence currently weights normalized platform, orientation-independent screen geometry, color depth, pixel ratio, CPU count, memory hint, touch points, WebGL vendor/renderer, and hashed font availability. Browser evidence weights user agent, vendor, languages, timezone, canvas, WebGL extensions, audio, and font metrics. Each response includes the evidence actually compared, its weight, and its similarity.

The initial conservative policy is:

- best similarity score at least `0.80`;
- available evidence coverage at least `0.45`;
- confidence at least `0.70`;
- best-to-runner-up margin at least `0.08` when another strong candidate exists.

Confidence combines similarity with evidence coverage and is penalized when the runner-up is close. These constants are explicit starting values for evaluation; they are not statistically calibrated yet. Missing values do not count as mismatches, but they reduce coverage. Device and browser histories are kept in separate candidate pools.

### Persistence model

SQLite stores two records:

- `Fingerprint` contains the opaque visitor ID, scope, timestamps, and observation count.
- `Observation` contains an immutable snapshot ID, scope, server observation time, and JSON-encoded schema-v3 browser signals.

Client timestamps are retained as snapshot metadata but matching persistence uses server time. Ambiguous requests are intentionally not attached to any history.

The current candidate scan is exact and bounded only by the number of stored observations. It is suitable for learning, evaluation, and modest deployments. Large installations still need indexed candidate buckets or approximate-nearest-neighbor retrieval before scoring.

### Current server security boundary

The server recomputes snapshot IDs and imposes strict JSON and size validation, but client browser signals remain attacker-controlled. Authentication, signed challenges, nonce/replay protection, rate limiting, trusted proxy configuration, and server-observed HTTP/TLS evidence remain planned. Do not use the current visitor ID as sole authorization or fraud proof.

## Build and test

```bash
go test ./...
go vet ./...
bash ./scripts/build.sh
```

On Windows:

```powershell
go test ./...
go vet ./...
./scripts/build.ps1
```

The generated browser artifact is `client/public/main.wasm`.

## Evaluation criteria

The repository includes a reproducible Playwright harness that collects repeated normal-session, isolated-context, cross-engine, and emulated-mobile observations. It replays those observations through the production matcher and reports:

- false-match rate: two different installations incorrectly receive one visitor ID;
- false-new rate: one installation incorrectly receives a new visitor ID;
- coverage and latency by browser and operating system;
- stability across browser updates, restarts, private browsing, and cleared storage;
- results with privacy defenses and deliberate signal tampering;
- dataset size, collection period, matching threshold, and confidence calibration.

Uniqueness alone is insufficient: a highly unique but unstable hash is a poor long-term identifier.

Install the pinned Playwright package and browser builds, then run the full matrix:

```bash
npm ci
npx playwright install chromium firefox webkit
bash ./scripts/build.sh
npm run evaluate
```

On Linux CI, add `--with-deps` to the installation command. The default report is written to `evaluation/reports/latest.json`; generated reports are ignored by Git because they contain raw test snapshots and environment-specific results.

Useful options include:

```bash
SIGIL_EVALUATION_BROWSERS=chromium,firefox,webkit \
SIGIL_EVALUATION_MODE=device \
SIGIL_EVALUATION_REPEATS=3 \
npm run evaluate
```

The harness labels desktop observations from all three engines and isolated contexts as one expected physical host. An emulated mobile context is a synthetic negative control. Reports explicitly state that controlled browser builds, a single CI runner, and emulation do not establish population-level accuracy.

The full browser job runs every Monday and can be launched manually with GitHub Actions `workflow_dispatch`. Its JSON report is retained as a workflow artifact for 30 days. Unit tests exercise the replay accounting on every normal CI run.

Run the workflow locally with:

```powershell
act --workflows .github/workflows/ci.yml
```

The artifact upload is skipped under `act` because its local runner does not provide GitHub's artifact service token.

GitHub reads `.github/dependabot.yml` directly. Dependabot CLI does not: its `-f` option expects a low-level update-job document. To exercise each configured ecosystem against the local checkout, use its internal package-manager name:

```powershell
dependabot update github_actions z46-dev/sigil --local .
dependabot update go_modules z46-dev/sigil --local .
dependabot update npm_and_yarn z46-dev/sigil --local .
```

### Initial harness baseline

The first completed local run on Windows used Playwright 1.62.1 with Chrome for Testing 151, Firefox 153, and WebKit 26.5. It collected 12 observations: two repeated session visits, one isolated-context visit, and one emulated-mobile visit per engine.

| Metric | Result |
| --- | ---: |
| Observations | 12 |
| Returning attempts | 10 |
| Correct decisions | 8 |
| Accuracy | 66.7% |
| False-match rate | 0% |
| False-new rate | 40% |
| Ambiguous rate | 0% |

Repeated and isolated contexts were stable within each engine. Cross-engine transitions into Firefox and WebKit were generally classified as new, as were the cross-engine synthetic-mobile transitions. This baseline demonstrates that the present device matcher favors avoiding false merges but does not yet provide acceptable cross-browser retention. It is a diagnostic result from one machine, not a population benchmark.

## Privacy and acceptable use

Browser fingerprints can become personal data and can be used for covert tracking. Deploy Sigil only with a documented lawful purpose, appropriate disclosure or consent, data minimization, retention limits, access controls, and a deletion mechanism. Requirements vary by jurisdiction; this project does not provide legal advice.

The project will not add permission-prompting probes, exploit browser vulnerabilities, or claim to bypass browser privacy protections. Risk signals should support an application decision rather than serve as sole proof that a person is malicious.

## License

Sigil is licensed under the [GNU Affero General Public License v3](LICENSE).
