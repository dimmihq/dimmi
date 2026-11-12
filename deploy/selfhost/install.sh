#!/usr/bin/env bash
# Installs and runs dimmi on one server with Docker Compose. Run as root:
#
#   curl -fsSL https://raw.githubusercontent.com/dimmihq/dimmi/main/deploy/selfhost/install.sh \
#     | bash -s -- --domain support.example.com --email you@example.com
#
# With no command it does everything: prepares the server, writes the
# settings, starts the stack and checks the domain. Safe to run again:
# secrets are made once and kept. After the first run it lives at
# /opt/dimmi/install.sh, also linked as `dimmictl`. Commands:
#
#   install   prepare, configure, start and check (the default)
#   prepare   Docker, firewall, swap, automatic security updates
#   configure fetch the stack's files and write .env
#   start     pull images, migrate and start everything
#   check     check DNS, HTTPS and /up
#   admin     create the first account: admin --email E --name N --workspace W
#   set       add or change settings: set KEY=value ... (or lines on stdin)
#   update    pull the newest image, migrate and restart
#   status    show the services
#   logs      follow the logs: logs [service]

set -euo pipefail

DIR="${DIMMI_DIR:-/opt/dimmi}"
REF="${DIMMI_REF:-main}"
RAW="https://raw.githubusercontent.com/dimmihq/dimmi/$REF/deploy/selfhost"
FILES=(compose.yaml Caddyfile postgres-init.sh .env.example install.sh)

domain="" email="" name="" workspace="" image="" clamav="" skip_prepare="" local_tls=""

say() { printf '\033[1m==> %s\033[0m\n' "$*"; }
warn() { printf '\033[33mwarning:\033[0m %s\n' "$*" >&2; }
die() { printf '\033[31merror:\033[0m %s\n' "$*" >&2; exit 1; }
random() { openssl rand -hex 24; }
compose() { docker compose --project-directory "$DIR" -f "$DIR/compose.yaml" "$@"; }

# Reads one value from .env, empty when unset.
setting() { [ -f "$DIR/.env" ] && sed -n "s/^$1=//p" "$DIR/.env" | tail -n1 || true; }

# Sets KEY=value in .env, replacing an existing line.
put() {
  local key="${1%%=*}" tmp
  tmp="$(mktemp "$DIR/.env.XXXXXX")"
  awk -v key="$key" -v line="$1" 'index($0, key "=") == 1 { if (!done) print line; done = 1; next }
    { print } END { if (!done) print line }' "$DIR/.env" >"$tmp"
  chmod 600 "$tmp"
  mv "$tmp" "$DIR/.env"
}

need_root() { [ "$(id -u)" = 0 ] || die "run this as root (sudo -i, or prefix with sudo)"; }

prepare() {
  need_root
  if ! command -v apt-get >/dev/null; then
    warn "not Debian or Ubuntu: install Docker yourself, then use --skip-prepare"
    return
  fi
  export DEBIAN_FRONTEND=noninteractive

  say "Updating packages"
  apt-get update -qq
  apt-get -y -qq -o Dpkg::Options::=--force-confold upgrade
  apt-get -y -qq install ca-certificates curl openssl ufw fail2ban unattended-upgrades >/dev/null
  systemctl enable --now unattended-upgrades fail2ban >/dev/null 2>&1 || true
  echo 'APT::Periodic::Update-Package-Lists "1";
APT::Periodic::Unattended-Upgrade "1";' >/etc/apt/apt.conf.d/20auto-upgrades

  if ! command -v docker >/dev/null; then
    say "Installing Docker"
    curl -fsSL https://get.docker.com | sh >/dev/null
  fi
  if [ ! -f /etc/docker/daemon.json ]; then
    echo '{"log-driver":"local","log-opts":{"max-size":"20m","max-file":"5"}}' >/etc/docker/daemon.json
    systemctl restart docker
  fi
  systemctl enable --now docker >/dev/null 2>&1 || true

  if [ -z "$(swapon --show --noheadings)" ] && [ ! -f /swapfile ]; then
    say "Adding 2 GB of swap"
    fallocate -l 2G /swapfile && chmod 600 /swapfile && mkswap /swapfile >/dev/null && swapon /swapfile
    grep -q '^/swapfile ' /etc/fstab || echo '/swapfile none swap sw 0 0' >>/etc/fstab
    echo 'vm.swappiness = 10' >/etc/sysctl.d/90-dimmi.conf
    sysctl --system >/dev/null
  fi

  # Password logins go only when a key can already log in, so nobody is locked out.
  local keys="/root/.ssh/authorized_keys"
  [ -n "${SUDO_USER:-}" ] && keys="$(getent passwd "$SUDO_USER" | cut -d: -f6)/.ssh/authorized_keys"
  if [ -s "$keys" ] && [ -d /etc/ssh/sshd_config.d ]; then
    printf 'PasswordAuthentication no\nKbdInteractiveAuthentication no\nPermitRootLogin prohibit-password\n' \
      >/etc/ssh/sshd_config.d/10-dimmi.conf
    sshd -t && { systemctl reload ssh 2>/dev/null || systemctl reload sshd 2>/dev/null || true; }
  fi

  say "Opening the firewall for SSH, HTTP and HTTPS"
  local ssh_port
  ssh_port="$(sshd -T 2>/dev/null | awk '/^port /{print $2; exit}')"
  ufw allow "${ssh_port:-22}/tcp" >/dev/null
  ufw allow 80/tcp >/dev/null
  ufw allow 443 >/dev/null
  ufw --force enable >/dev/null
}

# Copies the stack's files from beside this script when it runs from a
# checkout, else downloads them. With "fresh" it always downloads them.
fetch() {
  local here="" src="${BASH_SOURCE[0]:-}" f
  [ -n "$src" ] && [ -f "$src" ] && here="$(dirname "$(readlink -f "$src")")"
  mkdir -p "$DIR"
  chmod 700 "$DIR"
  for f in "${FILES[@]}"; do
    if [ "${1:-}" != fresh ] && [ -n "$here" ] && [ "$here" != "$DIR" ] && [ -f "$here/compose.yaml" ]; then
      cp "$here/$f" "$DIR/$f"
    elif [ "${1:-}" = fresh ] || [ ! -f "$DIR/$f" ]; then
      curl -fsSL "$RAW/$f" -o "$DIR/$f.new" || die "could not download $f from $RAW"
      mv "$DIR/$f.new" "$DIR/$f"
    fi
  done
  chmod +x "$DIR/install.sh" "$DIR/postgres-init.sh"
  if [ "$(id -u)" = 0 ] && [ -d /usr/local/bin ]; then ln -sf "$DIR/install.sh" /usr/local/bin/dimmictl; fi
}

configure() {
  command -v openssl >/dev/null || die "openssl is needed to make secrets"
  say "Writing the stack to $DIR"
  fetch

  [ -n "$domain" ] || domain="$(setting APP_DOMAIN)"
  [ -n "$email" ] || email="$(setting ACME_EMAIL)"
  [ -n "$domain" ] || die "pass --domain, the address dimmi will answer on, such as support.example.com"
  [ -n "$email" ] || die "pass --email, where Let's Encrypt sends certificate notices"
  case "$domain" in http*|*/*) die "--domain is a host name only, such as support.example.com";; esac

  if [ ! -f "$DIR/.env" ]; then
    say "Making secrets"
    umask 077
    cat >"$DIR/.env" <<ENV
# dimmi settings. install.sh made the secrets below once; keep them, since
# the database and files are locked with them. Add DIMMI_ settings under
# them, or use: dimmictl set KEY=value. See .env.example.
APP_DOMAIN=$domain
ACME_EMAIL=$email
IMAGE=${image:-ghcr.io/dimmihq/dimmi:self-host}
POSTGRES_PASSWORD=$(random)
POSTGRES_OWNER_PASSWORD=$(random)
POSTGRES_APP_PASSWORD=$(random)
VALKEY_PASSWORD=$(random)
S3_ACCESS_KEY=$(openssl rand -hex 10)
S3_SECRET_KEY=$(random)
DIMMI_APP__KEY=base64:$(openssl rand -base64 32)
ENV
  fi
  put "APP_DOMAIN=$domain"
  put "ACME_EMAIL=$email"
  [ -z "$image" ] || put "IMAGE=$image"
  [ -z "$local_tls" ] || put "CADDY_TLS=tls internal"
  if [ -n "$clamav" ]; then
    put "COMPOSE_PROFILES=clamav"
    put "DIMMI_MALWARE__DEFAULT=clamav"
    put "DIMMI_MALWARE__SCANNERS__CLAMAV__DRIVER=clamav"
    put "DIMMI_MALWARE__SCANNERS__CLAMAV__ADDRESS=clamav:3310"
  fi
}

wait_healthy() {
  local id status
  for _ in $(seq 1 90); do
    id="$(compose ps -q web 2>/dev/null || true)"
    status="$([ -n "$id" ] && docker inspect -f '{{.State.Health.Status}}' "$id" 2>/dev/null || true)"
    [ "$status" = healthy ] && return 0
    sleep 2
  done
  compose logs --tail 40 web >&2 || true
  die "the web service did not become healthy; see the logs above"
}

start() {
  [ -f "$DIR/.env" ] || die "no $DIR/.env yet: run install or configure first"
  say "Starting dimmi (the first run pulls images and migrates the database)"
  compose up -d --pull missing --remove-orphans || {
    compose logs --tail 60 migrate buckets >&2 || true
    die "the stack did not start; see the logs above"
  }
  wait_healthy
  say "dimmi is running"
}

check() {
  local host ip="" resolved
  host="$(setting APP_DOMAIN)"
  [ -n "$host" ] || die "no APP_DOMAIN in $DIR/.env"
  [ -n "$local_tls" ] || ip="$(curl -4 -fsS --max-time 5 https://api.ipify.org 2>/dev/null || true)"
  resolved="$(getent ahostsv4 "$host" 2>/dev/null | awk 'NR==1{print $1}' || true)"
  if [ -n "$ip" ] && [ "$resolved" != "$ip" ]; then
    warn "$host points to '${resolved:-nothing}', not this server ($ip)."
    echo "  Add a DNS record at your DNS provider, then run: dimmictl check"
    echo "    Type: A   Name: $host   Value: $ip   TTL: 300"
    echo "  Turn off any proxy (such as Cloudflare's orange cloud) until HTTPS works."
    return 1
  fi
  say "Checking https://$host/up"
  for _ in $(seq 1 30); do
    if curl -fsS --max-time 10 --resolve "$host:443:127.0.0.1" ${local_tls:+-k} "https://$host/up" >/dev/null 2>&1; then
      say "HTTPS works at https://$host. Create the first account with: dimmictl admin --help"
      return 0
    fi
    sleep 4
  done
  compose logs --tail 30 caddy >&2 || true
  warn "https://$host/up did not answer yet; Caddy may still be getting the certificate (logs above)"
  return 1
}

set_values() {
  [ -f "$DIR/.env" ] || die "no $DIR/.env yet: run install first"
  local line n=0
  if [ $# -eq 0 ]; then
    while IFS= read -r line; do
      [[ "$line" =~ ^[A-Za-z_][A-Za-z0-9_]*= ]] || continue
      put "$line"
      n=$((n + 1))
    done
  fi
  for line in "$@"; do
    [[ "$line" =~ ^[A-Za-z_][A-Za-z0-9_]*= ]] || die "not KEY=value: $line"
    put "$line"
    n=$((n + 1))
  done
  say "Saved $n setting(s); restarting the app"
  compose up -d --force-recreate web worker
  wait_healthy
}

# New installs are invite-only: the first account comes from here, owns a
# new workspace, and invites everyone else from the app.
admin() {
  [ -n "$email" ] && [ -n "$name" ] && [ -n "$workspace" ] || die "usage: admin --email E --name N --workspace W"
  compose exec -T web /dimmi user:create --no-interaction --email "$email" --name "$name" \
    --workspace "$workspace" --generate-password
  say "Sign in at https://$(setting APP_DOMAIN) with the password above, then change it."
}

update() {
  say "Fetching the newest stack files from $RAW"
  fetch fresh
  say "Pulling the newest images"
  compose pull --ignore-pull-failures
  start
}

usage() {
  local src="${BASH_SOURCE[0]:-}"
  if [ -n "$src" ] && [ -f "$src" ]; then sed -n '2,22p' "$src" | sed 's/^# \{0,1\}//'
  else echo "Usage: https://github.com/dimmihq/dimmi/blob/main/deploy/selfhost/SKILL.md"; fi
  exit "${1:-0}"
}

cmd="install"
case "${1:-}" in
  install|prepare|configure|start|check|admin|set|update|status|logs|help) cmd="$1"; shift;;
esac
if [ "$cmd" != set ] && [ "$cmd" != logs ]; then
  while [ $# -gt 0 ]; do
    case "$1" in
      --domain) domain="${2:?}"; shift 2;;
      --email) email="${2:?}"; shift 2;;
      --name) name="${2:?}"; shift 2;;
      --workspace) workspace="${2:?}"; shift 2;;
      --image) image="${2:?}"; shift 2;;
      --dir) DIR="${2:?}"; shift 2;;
      --clamav) clamav=1; shift;;
      --skip-prepare) skip_prepare=1; shift;;
      --local-tls) local_tls=1; shift;;
      -h|--help) usage;;
      *) die "unknown option: $1 (see --help)";;
    esac
  done
fi
[ -z "$local_tls" ] && [ "$(setting CADDY_TLS)" = "tls internal" ] && local_tls=1

case "$cmd" in
  install)
    [ -n "$skip_prepare" ] || prepare
    configure
    start
    check || true
    ;;
  prepare) prepare;;
  configure) configure;;
  start) start;;
  check) check;;
  admin) admin;;
  set) set_values "$@";;
  update) update;;
  status) compose ps;;
  logs) compose logs -f --tail 100 "$@";;
  help) usage;;
esac
