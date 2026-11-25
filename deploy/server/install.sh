#!/usr/bin/env bash
# Wipes the server and installs Ubuntu on an encrypted disk. Everything on
# both drives is lost. Boot the server into the Hetzner rescue system with
# your SSH key first, then run this from the repository, giving the disk
# passphrase on standard input:
#
#   deploy/server/install.sh root@<server> < passphrase-file
#
# The passphrase is one line. It goes straight to the rescue system's memory
# and is never written to a disk or printed. When this finishes, restart the
# server, unlock it (see post-install.sh) and run prepare.sh.

set -euo pipefail

SERVER="${1:?usage: $0 root@<server> < passphrase-file}"
HERE="$(cd "$(dirname "$0")" && pwd)"
SSH=(ssh -o StrictHostKeyChecking=accept-new -o UserKnownHostsFile=/dev/null "$SERVER")

IFS= read -r PASSPHRASE || true
[ "${#PASSPHRASE}" -ge 20 ] || { echo "the passphrase must be one line of at least 20 characters" >&2; exit 1; }

"${SSH[@]}" 'test -x /root/.oldroot/nfs/install/installimage' || { echo "$SERVER is not in the rescue system" >&2; exit 1; }

"${SSH[@]}" 'cat >/root/install.conf' <"$HERE/install.conf"
"${SSH[@]}" 'cat >/root/post-install && chmod 0755 /root/post-install' <"$HERE/post-install.sh"
printf 'CRYPTPASSWORD %s\n' "$PASSPHRASE" | "${SSH[@]}" 'umask 077 && cat >>/root/install.conf'

"${SSH[@]}" 'bash -s' <<'REMOTE'
set -euo pipefail

for array in /dev/md?*; do
  [ -b "$array" ] && mdadm --stop "$array" || true
done

for drive in /dev/nvme0n1 /dev/nvme1n1; do
  wipefs -a "$drive"?* "$drive" || true
  nvme format "$drive" --ses=1 --force || blkdiscard -f "$drive"
done

/root/.oldroot/nfs/install/installimage -a -c /root/install.conf -x /root/post-install </dev/null
shred -u /root/install.conf
REMOTE

echo "Installed. Restart the server, then unlock it: ssh -p 2222 $SERVER cryptroot-unlock"
