#!/usr/bin/env bash
# Prepares a fresh Hetzner Ubuntu install for k3s. Run as root on the server:
#
#   ssh root@<server> bash -s < deploy/server/prepare.sh
#
# Safe to run again. It hardens SSH, turns swap off and reuses the swap array
# (md0) as the RAID1 volume /srv/storage, loads the kernel settings k3s needs,
# installs the AppArmor profile pods need and opens the firewall for SSH,
# HTTP, HTTPS, the Kubernetes API and the private network.

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

install -m 0644 /dev/stdin /etc/apparmor.d/cri-containerd.apparmor.d <<'EOF'
# containerd's default container profile, with one change: processes in a
# pod may signal each other when Ubuntu stacks the profile as
# "cri-containerd.apparmor.d//&unconfined". Without it, every signal between
# two processes of one pod is denied, so Postgres IO workers never wake,
# timeouts never fire and graceful stops hang.
#
# containerd keeps a profile of this name when one is already loaded, and
# AppArmor loads this one from /etc/apparmor.d at boot, before k3s starts.

#include <tunables/global>

profile cri-containerd.apparmor.d flags=(attach_disconnected,mediate_deleted) {
  #include <abstractions/base>

  network,
  capability,
  file,
  umount,

  signal (receive) peer=unconfined,
  signal (receive) peer=runc,
  signal (receive) peer=crun,
  signal (send,receive) peer=cri-containerd.apparmor.d,
  signal (send,receive) peer="cri-containerd.apparmor.d//&unconfined",

  deny @{PROC}/* w,
  deny @{PROC}/{[^1-9],[^1-9][^0-9],[^1-9s][^0-9y][^0-9s],[^1-9][^0-9][^0-9][^0-9/]*}/** w,
  deny @{PROC}/sys/[^k]** w,
  deny @{PROC}/sys/kernel/{?,??,[^s][^h][^m]**} w,
  deny @{PROC}/sysrq-trigger rwklx,
  deny @{PROC}/kcore rwklx,

  deny mount,

  deny /sys/[^f]*/** wklx,
  deny /sys/f[^s]*/** wklx,
  deny /sys/fs/[^c]*/** wklx,
  deny /sys/fs/c[^g]*/** wklx,
  deny /sys/fs/cg[^r]*/** wklx,
  deny /sys/firmware/** rwklx,
  deny /sys/devices/virtual/powercap/** rwklx,
  deny /sys/kernel/security/** rwklx,

  ptrace (trace,read,tracedby,readby) peer=cri-containerd.apparmor.d,
  ptrace (trace,read,tracedby,readby) peer="cri-containerd.apparmor.d//&unconfined",
}
EOF
apparmor_parser -r -W /etc/apparmor.d/cri-containerd.apparmor.d

sed -i 's/^DEFAULT_FORWARD_POLICY=.*/DEFAULT_FORWARD_POLICY="ACCEPT"/' /etc/default/ufw
ufw --force reset >/dev/null
ufw default deny incoming
ufw default allow outgoing
for p in 22/tcp 80/tcp 443/tcp 6443/tcp; do ufw allow "$p"; done
ufw allow from 10.0.0.0/8
ufw --force enable
ufw status verbose
