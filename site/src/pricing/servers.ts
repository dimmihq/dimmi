/*
 * Server sizes for a self-hosted install, priced at DigitalOcean's basic
 * droplets with Hetzner's shared cloud beside them. The smallest is the
 * install guide's minimum; malware scanning needs about 2 GB more memory.
 */
export const SERVERS = [
  {
    name: "Minimum",
    price: "$24",
    where: "DigitalOcean, or about €20 at Hetzner",
    specs: [
      ["CPU", "2 vCPU"],
      ["Memory", "4 GB"],
      ["Disk", "40 GB"],
    ],
    fits: "Trying dimmi out and small teams. Leaves malware scanning off.",
  },
  {
    name: "Recommended",
    price: "$48",
    where: "DigitalOcean, or about €36 at Hetzner",
    specs: [
      ["CPU", "4 vCPU"],
      ["Memory", "8 GB"],
      ["Disk", "80 GB"],
    ],
    fits: "Most teams, with malware scanning on and room for attachments and search.",
  },
  {
    name: "Large",
    price: "$96",
    where: "DigitalOcean, or about €70 at Hetzner",
    specs: [
      ["CPU", "8 vCPU"],
      ["Memory", "16 GB"],
      ["Disk", "160 GB"],
    ],
    fits: "Busy teams with many channels, large help centers and heavy AI traffic.",
  },
] as const;
