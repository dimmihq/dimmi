#!/usr/bin/env bash
# Runs inside the new system at the end of installimage. It puts a small SSH
# server in the boot image, so the encrypted disk can be unlocked from afar:
#
#   ssh -p 2222 root@<server> cryptroot-unlock
#
# The boot image takes the same keys as root, and only lets them unlock.

set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

test -s /root/.ssh/authorized_keys || { echo "no SSH keys for root; refusing to make a server nobody can unlock" >&2; exit 1; }

apt-get update -qq
apt-get -y -qq install cryptsetup-initramfs dropbear-initramfs

install -d -m 0700 /etc/dropbear/initramfs
install -m 0600 /root/.ssh/authorized_keys /etc/dropbear/initramfs/authorized_keys
cat >/etc/dropbear/initramfs/dropbear.conf <<'EOF'
DROPBEAR_OPTIONS="-p 2222 -s -j -k -I 300 -c cryptroot-unlock"
EOF

cat >/etc/initramfs-tools/conf.d/unlock-network <<'EOF'
IP=dhcp
EOF

update-initramfs -u -k all
