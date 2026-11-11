# The server

One Hetzner dedicated server runs every environment. The files the steps copy live in
[`deploy/server`](../../deploy/server).

| Part | What it is |
|---|---|
| Server `dimmi-1` | Xeon E-2276G, 64 GB ECC, 2 × 960 GB datacenter NVMe, FSN1. Public address in the Robot console, private `10.10.1.2` |
| Disks | Software RAID1: `/` 861 GB (md2), `/boot` (md1), `/srv/storage` 32 GB (md0, the former swap). No swap |
| vSwitch `dimmi` | Robot vSwitch, VLAN 4000, with the server added |
| Cloud network `dimmi` | `10.10.0.0/16`: cloud subnet `10.10.0.0/24`, vSwitch subnet `10.10.1.0/24` (gateway `10.10.1.1`) |
| Load balancer `dimmi` | lb11 in fsn1. Public `46.224.169.155`, private `10.10.0.2`. TCP 80 and 443 with the PROXY protocol, to `10.10.1.2` |
| k3s | Stable channel, node IP `10.10.1.2`, flannel on `vlan4000`, Traefik and the local-path provisioner |

The load balancer passes the real client address through the PROXY protocol, and Traefik trusts that header only from
private addresses. Both environments share the load balancer; Traefik routes each host to its namespace.

## Firewall

`ufw` allows SSH, HTTP, HTTPS and the Kubernetes API (6443) from anywhere, and everything from `10.0.0.0/8`, which covers
the private network, pods and services. Routed traffic is allowed so pods can reach each other.

## Set it up from nothing

In the commands below `$SERVER` is the server's public address, shown in the Robot console.

1. **HUMAN, Robot.** Install Ubuntu with `installimage`, RAID1, with your SSH key.
2. **HUMAN, Robot.** Create the vSwitch `dimmi` with VLAN 4000 and add the server.
3. **AUTOMATED.** Prepare the system:

   ```sh
   ssh root@$SERVER bash -s < deploy/server/prepare.sh
   ssh root@$SERVER reboot
   ```

4. **AUTOMATED, Hetzner Cloud API.** Create the network with its cloud subnet, add the vSwitch subnet with the vSwitch's
   id, then create the load balancer on the network with its two services and the IP target `10.10.1.2`.
5. **AUTOMATED.** Bring up the private interface. It answers once the vSwitch attachment settles, which can take a few
   minutes:

   ```sh
   scp deploy/server/vswitch.yaml root@$SERVER:/etc/netplan/60-vswitch.yaml
   ssh root@$SERVER 'chmod 600 /etc/netplan/60-vswitch.yaml && netplan apply && ping -c3 10.10.1.1'
   ```

6. **AUTOMATED.** Install k3s:

   ```sh
   ssh root@$SERVER 'mkdir -p /etc/rancher/k3s /var/lib/rancher/k3s/server/manifests'
   sed "s/SERVER_PUBLIC_IP/$SERVER/" deploy/server/k3s-config.yaml | ssh root@$SERVER "cat > /etc/rancher/k3s/config.yaml"
   scp deploy/server/traefik-config.yaml root@$SERVER:/var/lib/rancher/k3s/server/manifests/
   ssh root@$SERVER 'curl -sfL https://get.k3s.io | INSTALL_K3S_CHANNEL=stable sh -'
   ```

Check: `kubectl get nodes -o wide` shows `dimmi-1` Ready with internal IP `10.10.1.2`, and the load balancer's targets are
healthy on both ports. A request to `http://46.224.169.155/` answers 404 from Traefik until something is routed.

## Data on this one machine

RAID1 survives one disk. Nothing here survives losing the server: only off-site backups do, as
[`deploy/k8s`](../../deploy/k8s/README.md) describes.
