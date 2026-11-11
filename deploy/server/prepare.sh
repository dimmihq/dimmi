#!/usr/bin/env bash
# Prepares a fresh Hetzner Ubuntu install for k3s. Run as root on the server:
#
#   ssh root@<server> bash -s < deploy/server/prepare.sh
#
# Safe to run again. It hardens SSH, turns swap off and reuses the swap array
# (md0) as the RAID1 volume /srv/storage, loads the kernel settings k3s needs
# and opens the firewall for SSH, HTTP, HTTPS, the Kubernetes API and the
# private network.

set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

hostnamectl set-hostname dimmi-1
grep -q dimmi-1 /etc/hosts || sed -i "s/^\(127\.0\.1\.1\s\).*/\1dimmi-1/" /etc/hosts
grep -q dimmi-1 /etc/hosts || echo "127.0.1.1 dimmi-1" >> /etc/hosts
timedatectl set-timezone UTC

apt-get update -qq
apt-get -y -qq -o Dpkg::Options::=--force-confold full-upgrade
apt-get -y -qq install ufw smartmontools nvme-cli jq curl nfs-common open-iscsi unattended-upgrades fail2ban
systemctl enable --now unattended-upgrades smartmontools fstrim.timer

cat >/etc/ssh/sshd_config.d/10-hardening.conf <<'EOF'
PasswordAuthentication no
KbdInteractiveAuthentication no
PermitRootLogin prohibit-password
PubkeyAuthentication yes
EOF
sshd -t && { systemctl reload ssh 2>/dev/null || true; }

swapoff -a
sed -i '/[[:space:]]swap[[:space:]]/d' /etc/fstab
sed -i '\|^# /dev/md/0$|d' /etc/fstab
if ! blkid -o value -s TYPE /dev/md0 | grep -q ext4; then
  wipefs -a /dev/md0
  mkfs.ext4 -q -L storage -m 0 /dev/md0
fi
mkdir -p /srv/storage
uuid=$(blkid -o value -s UUID /dev/md0)
grep -q "$uuid" /etc/fstab || echo "UUID=$uuid /srv/storage ext4 defaults,noatime 0 2" >> /etc/fstab
systemctl daemon-reload
mount -a

cat >/etc/modules-load.d/k3s.conf <<'EOF'
overlay
br_netfilter
EOF
modprobe overlay
modprobe br_netfilter

cat >/etc/sysctl.d/90-k3s.conf <<'EOF'
net.ipv4.ip_forward = 1
net.ipv6.conf.all.forwarding = 1
net.bridge.bridge-nf-call-iptables = 1
net.bridge.bridge-nf-call-ip6tables = 1
fs.inotify.max_user_instances = 8192
fs.inotify.max_user_watches = 524288
fs.file-max = 2097152
vm.max_map_count = 262144
vm.swappiness = 0
kernel.panic = 10
kernel.panic_on_oops = 1
EOF
sysctl --system >/dev/null

sed -i 's/^DEFAULT_FORWARD_POLICY=.*/DEFAULT_FORWARD_POLICY="ACCEPT"/' /etc/default/ufw
ufw --force reset >/dev/null
ufw default deny incoming
ufw default allow outgoing
for p in 22/tcp 80/tcp 443/tcp 6443/tcp; do ufw allow "$p"; done
ufw allow from 10.0.0.0/8
ufw --force enable
ufw status verbose
