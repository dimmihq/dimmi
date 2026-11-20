/*
 * The hero network's layout, shared by the prerendered SVG poster and the
 * three.js scene so both draw the same picture. Customers sit on a loose
 * shell around the dimmi mark; a few teammates sit among them. The layout
 * is seeded, so every build and every visit draws the same network.
 */
export type NetworkNode = {
  x: number;
  y: number;
  z: number;
  kind: "customer" | "teammate";
};

function seeded(seed: number) {
  let a = seed;
  return () => {
    a |= 0;
    a = (a + 0x6d2b79f5) | 0;
    let t = Math.imul(a ^ (a >>> 15), 1 | a);
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

export const NODE_COUNT = 96;
export const TEAMMATE_EVERY = 12;

export function buildNetwork(): NetworkNode[] {
  const random = seeded(7);
  const nodes: NetworkNode[] = [];
  const golden = Math.PI * (3 - Math.sqrt(5));
  for (let i = 0; i < NODE_COUNT; i++) {
    const y = 1 - (i / (NODE_COUNT - 1)) * 2;
    const ring = Math.sqrt(1 - y * y);
    const theta = golden * i + random() * 0.35;
    const radius = 0.62 + random() * 0.48;
    nodes.push({
      x: Math.cos(theta) * ring * radius,
      y: y * radius * 0.92,
      z: Math.sin(theta) * ring * radius,
      kind: i % TEAMMATE_EVERY === 5 ? "teammate" : "customer",
    });
  }
  return nodes;
}

/* Rotates a point around the y axis, then tilts it toward the viewer. */
export function project(node: NetworkNode, angle: number, tilt = 0.32) {
  const cos = Math.cos(angle);
  const sin = Math.sin(angle);
  const x = node.x * cos + node.z * sin;
  const z1 = -node.x * sin + node.z * cos;
  const y = node.y * Math.cos(tilt) - z1 * Math.sin(tilt);
  const z = node.y * Math.sin(tilt) + z1 * Math.cos(tilt);
  const camera = 4.2;
  const scale = camera / (camera - z);
  return { x: x * scale, y: y * scale, z, scale };
}
