#!/usr/bin/env bash

set -euo pipefail

readonly service_name="sigil"
readonly service_user="sigil"
readonly service_group="sigil"
readonly install_dir="/opt/sigil"
readonly unit_path="/etc/systemd/system/${service_name}.service"
readonly repository_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"

overwrite_config=false
start_service=true

usage() {
    echo "Usage: sudo bash ./scripts/install-systemd.sh [--overwrite-config] [--no-start]"
}

for argument in "$@"; do
    case "$argument" in
        --overwrite-config)
            overwrite_config=true
            ;;
        --no-start)
            start_service=false
            ;;
        --help|-h)
            usage
            exit 0
            ;;
        *)
            usage >&2
            exit 2
            ;;
    esac
done

if [[ $EUID -ne 0 ]]; then
    echo "This installer must run as root." >&2
    exit 1
fi

for command_name in go install setcap getcap systemctl useradd groupadd getent mktemp; do
    if ! command -v "$command_name" >/dev/null 2>&1; then
        echo "Required command is unavailable: $command_name" >&2
        exit 1
    fi
done

if [[ ! -f "$repository_dir/config.toml" ]]; then
    echo "Missing $repository_dir/config.toml" >&2
    exit 1
fi

build_dir="$(mktemp -d)"
cleanup() {
    rm -rf -- "$build_dir"
}
trap cleanup EXIT

echo "Building Sigil..."
(
    cd -- "$repository_dir"
    go mod download
    go build -trimpath -ldflags="-s -w" -o "$build_dir/sigil" ./src
    install -d -m 0755 "$build_dir/public"
    cp -a ./client/public/. "$build_dir/public/"
    GOOS=js GOARCH=wasm go build -trimpath -o "$build_dir/public/main.wasm" ./client/src
)

if ! getent group "$service_group" >/dev/null; then
    groupadd --system "$service_group"
fi

if ! getent passwd "$service_user" >/dev/null; then
    useradd \
        --system \
        --gid "$service_group" \
        --home-dir "$install_dir" \
        --no-create-home \
        --shell /usr/sbin/nologin \
        "$service_user"
fi

if systemctl is-active --quiet "$service_name.service"; then
    systemctl stop "$service_name.service"
fi

install -d -o root -g "$service_group" -m 1770 "$install_dir"
install -d -o root -g "$service_group" -m 0750 "$install_dir/client/public"
install -o root -g "$service_group" -m 0750 "$build_dir/sigil" "$install_dir/sigil"
cp -a "$build_dir/public/." "$install_dir/client/public/"
chown -R root:"$service_group" "$install_dir/client"
find "$install_dir/client" -type d -exec chmod 0750 {} +
find "$install_dir/client/public" -type f -exec chmod 0640 {} +

if [[ -d "$repository_dir/data" ]]; then
    install -d -o root -g "$service_group" -m 0750 "$install_dir/data"
    cp -a "$repository_dir/data/." "$install_dir/data/"
    chown -R root:"$service_group" "$install_dir/data"
    find "$install_dir/data" -type d -exec chmod 0750 {} +
    find "$install_dir/data" -type f -exec chmod 0640 {} +
fi

if [[ ! -f "$install_dir/config.toml" || $overwrite_config == true ]]; then
    install -o root -g "$service_group" -m 0640 "$repository_dir/config.toml" "$install_dir/config.toml"
else
    echo "Preserving existing $install_dir/config.toml"
fi

for database_name in sigil.db ip-intelligence.db; do
    if [[ ! -e "$install_dir/$database_name" ]]; then
        if [[ -f "$repository_dir/$database_name" ]]; then
            install -o "$service_user" -g "$service_group" -m 0640 "$repository_dir/$database_name" "$install_dir/$database_name"
        else
            install -o "$service_user" -g "$service_group" -m 0640 /dev/null "$install_dir/$database_name"
        fi
    fi
done

setcap cap_net_bind_service=+ep "$install_dir/sigil"
if [[ $(getcap "$install_dir/sigil") != *"cap_net_bind_service=ep"* ]]; then
    echo "Failed to grant CAP_NET_BIND_SERVICE to $install_dir/sigil" >&2
    exit 1
fi

install -o root -g root -m 0644 /dev/stdin "$unit_path" <<'UNIT'
[Unit]
Description=Sigil browser fingerprinting service
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=sigil
Group=sigil
WorkingDirectory=/opt/sigil
ExecStart=/opt/sigil/sigil
Restart=on-failure
RestartSec=5s
UMask=0027

CapabilityBoundingSet=CAP_NET_BIND_SERVICE
PrivateDevices=true
PrivateTmp=true
ProtectClock=true
ProtectControlGroups=true
ProtectHome=true
ProtectHostname=true
ProtectKernelLogs=true
ProtectKernelModules=true
ProtectKernelTunables=true
ProtectSystem=strict
ReadWritePaths=/opt/sigil
RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6
RestrictNamespaces=true
RestrictRealtime=true
RestrictSUIDSGID=true
SystemCallArchitectures=native

[Install]
WantedBy=multi-user.target
UNIT

systemctl daemon-reload
systemctl enable "$service_name.service"

if [[ $start_service == true ]]; then
    systemctl restart "$service_name.service"
    systemctl --no-pager --full status "$service_name.service"
else
    echo "Installed without starting. Run: systemctl start $service_name.service"
fi

echo "Installed Sigil in $install_dir"
