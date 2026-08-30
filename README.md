# Sigil

![CI](https://img.shields.io/github/actions/workflow/status/z46-dev/sigil/ci.yml?branch=main&event=push&label=CI)
![Made with Go](https://img.shields.io/badge/-Made_with_Go-007d9c?logo=go&logoColor=white)

Sigil is an experimental, self-hosted browser fingerprinting service written in Go targeting WebAssembly. It collects permissionless browser signals, creates snapshot IDs, and uses server-side history to assign durable visitor IDs.

Sigil is a research-quality project, not a production fraud-prevention platform. Its current single-machine Playwright baseline achieved 66.7% accuracy with a 40% false-new rate. The API has strict validation, rate limits, same-origin checks, and network-bound single-use challenges, but browser signals remain attacker-controlled. Authentication and automatic retention are not implemented.

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
| Server/network signals | ⚠️ | ❌ | ❌ | ✅ | ✅ |
| Bot and fraud signals | ⚠️ | ❌ | ⚠️ | ✅ | ✅ |
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

### Install as a systemd service

On a Linux system with Go, systemd, and `setcap` installed, run:

```bash
sudo bash ./scripts/install-systemd.sh
```

The installer builds the server and WebAssembly client, creates a non-login `sigil` system user, and installs the binary, `client/public`, configuration, MMDB archives, and any existing databases under `/opt/sigil`. It grants only `CAP_NET_BIND_SERVICE` to the binary so the unprivileged service can listen on ports below 1024. The systemd unit is enabled and started automatically.

Reinstalling preserves `/opt/sigil/config.toml` and existing databases. To replace the deployed configuration with the repository copy, use:

```bash
sudo bash ./scripts/install-systemd.sh --overwrite-config
```

Use `--no-start` to install and enable the unit without starting it. If configuration uses custom database paths, ensure those files and their parent directories are writable by the `sigil` user. View service logs with `journalctl -u sigil -f`.

### Server/network signals

Server-observed network evidence is opt-in. When enabled, Sigil reduces the remote address to an IPv4 `/24` or IPv6 `/56` prefix and stores only an HMAC digest. The coarse digest contributes a small amount of matching evidence and also binds challenges to the requesting network. Raw IP addresses are not retained.

```toml
[web_server]
server_network_signals = true
network_signal_key = "replace-with-at-least-32-random-characters"
ip_data_directory = "data"
trusted_proxies = ["127.0.0.1"] # Only when HTTPS terminates at a local reverse proxy.

[ip_intelligence]
database_file = "ip-intelligence.db"
threatfox_auth_key = "" # Optional abuse.ch key.
```

Keep `network_signal_key` private and stable; changing it makes old network observations incomparable. List only proxy IPs or CIDRs you control in `trusted_proxies`. An empty list uses the direct TCP peer and ignores spoofable forwarding headers.

Place the downloaded `GeoLite2-ASN_YYYYMMDD.tar.gz`, `GeoLite2-City_YYYYMMDD.tar.gz`, and `GeoLite2-Country_YYYYMMDD.tar.gz` archives in `ip_data_directory`. Sigil selects the newest matching archive, reads its MMDB member directly into memory, and never writes an extracted copy to disk. The default `data` directory is ignored by Git. ASN organization, country, and city are returned as an `ip` classification and persisted with the server observation; the raw address is discarded after evaluation.

Sigil also maintains `ip-intelligence.db` separately from the visitor database. Published feeds are normalized into expiring CIDR indicators in SQLite, while request-time lookups use immutable in-memory prefix indexes. Successful updates atomically replace one source; failed or empty updates retain the last good snapshot. The updater currently consumes ProxyScrape, Tor Onionoo with a FireHOL Tor fallback, AWS, Google Cloud, Azure, Cloudflare, and FireHOL's proxy aggregate. ThreatFox malware indicators are enabled when `threatfox_auth_key` is configured.

Source schedules reflect their expected volatility: ProxyScrape every 15 minutes, Tor and ThreatFox hourly, cloud ranges every 12 hours, and FireHOL proxies daily. Classifications remain separate—`openProxy`, `tor`, `hosting`, and `malicious`—because a cloud address is not proof of VPN use or abuse. Feed data has its own upstream terms and may contain false positives; review each source's terms before commercial deployment.

Identification responses include an `aggressiveness` object with a `0`–`100` score, a `low`, `moderate`, or `high` level, and the number of signals used. This is a transparent privacy-intensity estimate based on available weighted probes; it is not an accuracy or uniqueness claim.

Challenges expire after two minutes, are single-use, are bound to the observed user agent and (when enabled) coarse network prefix, and identification rejects stale collection timestamps, mismatched user agents, unknown JSON fields, and snapshot IDs that do not match their signal content. HTTPS is still required to protect requests in transit. These controls detect accidental or in-transit alteration and basic replay; they cannot make data supplied by a malicious browser trustworthy.

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
