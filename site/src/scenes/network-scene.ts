/*
 * The hero's three.js scene, loaded after the page paints. Conversations
 * travel from customers into the dimmi mark at the centre. Most are
 * answered there (onResolved fires so the page can pulse the mark); the
 * rest are routed out to a teammate in the accent colour. The network
 * turns slowly and can be dragged. Colours come from BoardUI's tokens and
 * follow the theme.
 */
import {
  BufferAttribute,
  BufferGeometry,
  Color,
  Group,
  LineSegments,
  NormalBlending,
  PerspectiveCamera,
  Points,
  Scene,
  ShaderMaterial,
  Vector3,
  WebGLRenderer,
} from "three";

import { buildNetwork, type NetworkNode } from "./network";

type Palette = { ink: Color; faint: Color; accent: Color };

function cssColor(name: string): Color {
  const canvas = document.createElement("canvas");
  canvas.width = canvas.height = 1;
  const ctx = canvas.getContext("2d");
  const value = getComputedStyle(document.documentElement).getPropertyValue(name).trim();
  if (!ctx || !value) {
    return new Color(0x888888);
  }
  ctx.fillStyle = value;
  ctx.fillRect(0, 0, 1, 1);
  const [r, g, b] = ctx.getImageData(0, 0, 1, 1).data;
  return new Color(r / 255, g / 255, b / 255).convertSRGBToLinear();
}

function readPalette(): Palette {
  return {
    ink: cssColor("--color-text-primary"),
    faint: cssColor("--color-text-tertiary"),
    accent: cssColor("--color-accent-500"),
  };
}

const pointVertex = /* glsl */ `
  attribute float size;
  attribute float alpha;
  attribute vec3 tint;
  varying vec3 vTint;
  varying float vAlpha;
  uniform float uPixelRatio;
  void main() {
    vTint = tint;
    vec4 mv = modelViewMatrix * vec4(position, 1.0);
    float depth = clamp(1.0 - (-mv.z - 3.1) / 2.4, 0.25, 1.0);
    vAlpha = alpha * depth;
    gl_PointSize = size * uPixelRatio * (4.2 / -mv.z);
    gl_Position = projectionMatrix * mv;
  }
`;

const pointFragment = /* glsl */ `
  varying vec3 vTint;
  varying float vAlpha;
  void main() {
    float d = length(gl_PointCoord - 0.5);
    float a = smoothstep(0.5, 0.36, d);
    if (a < 0.01) discard;
    gl_FragColor = vec4(vTint, a * vAlpha);
  }
`;

const lineVertex = /* glsl */ `
  attribute float alpha;
  attribute vec3 tint;
  varying vec3 vTint;
  varying float vAlpha;
  void main() {
    vTint = tint;
    vec4 mv = modelViewMatrix * vec4(position, 1.0);
    float depth = clamp(1.0 - (-mv.z - 3.1) / 2.4, 0.2, 1.0);
    vAlpha = alpha * depth;
    gl_Position = projectionMatrix * mv;
  }
`;

const lineFragment = /* glsl */ `
  varying vec3 vTint;
  varying float vAlpha;
  void main() { gl_FragColor = vec4(vTint, vAlpha); }
`;

type Packet = {
  from: number;
  to: number;
  start: number;
  phase: "in" | "out";
  routed: boolean;
};

const IN_MS = 1500;
const OUT_MS = 1100;
const SPAWN_MS = 380;
const MAX_PACKETS = 24;

export function mountNetwork(canvas: HTMLCanvasElement, options: { onResolved: () => void; onRouted: () => void }) {
  const nodes: NetworkNode[] = buildNetwork();
  const teammates = nodes.map((n, i) => (n.kind === "teammate" ? i : -1)).filter((i) => i >= 0);
  const customers = nodes.map((n, i) => (n.kind === "customer" ? i : -1)).filter((i) => i >= 0);
  const positions = nodes.map((n) => new Vector3(n.x, n.y, n.z));

  const renderer = new WebGLRenderer({ canvas, antialias: true, alpha: true });
  const pixelRatio = Math.min(2, window.devicePixelRatio || 1);
  renderer.setPixelRatio(pixelRatio);
  const scene = new Scene();
  const camera = new PerspectiveCamera(34, 1, 0.1, 20);
  camera.position.set(0, 0, 4.2);

  const group = new Group();
  group.rotation.x = 0.32;
  scene.add(group);

  let palette = readPalette();

  // Spokes from the centre to every node.
  const spokeGeometry = new BufferGeometry();
  const spokePositions = new Float32Array(nodes.length * 6);
  const spokeAlpha = new Float32Array(nodes.length * 2);
  const spokeTint = new Float32Array(nodes.length * 6);
  positions.forEach((p, i) => {
    spokePositions.set([0, 0, 0, p.x, p.y, p.z], i * 6);
    spokeAlpha.set([0.02, 0.28], i * 2);
  });
  spokeGeometry.setAttribute("position", new BufferAttribute(spokePositions, 3));
  spokeGeometry.setAttribute("alpha", new BufferAttribute(spokeAlpha, 1));
  spokeGeometry.setAttribute("tint", new BufferAttribute(spokeTint, 3));
  const lineMaterial = new ShaderMaterial({
    vertexShader: lineVertex,
    fragmentShader: lineFragment,
    transparent: true,
    depthWrite: false,
    blending: NormalBlending,
  });
  group.add(new LineSegments(spokeGeometry, lineMaterial));

  // Nodes.
  const nodeGeometry = new BufferGeometry();
  const nodeAlpha = new Float32Array(nodes.length);
  const nodeSize = new Float32Array(nodes.length);
  const nodeTint = new Float32Array(nodes.length * 3);
  const glow = new Float32Array(nodes.length);
  nodeGeometry.setAttribute(
    "position",
    new BufferAttribute(new Float32Array(positions.flatMap((p) => [p.x, p.y, p.z])), 3),
  );
  nodeGeometry.setAttribute("alpha", new BufferAttribute(nodeAlpha, 1));
  nodeGeometry.setAttribute("size", new BufferAttribute(nodeSize, 1));
  nodeGeometry.setAttribute("tint", new BufferAttribute(nodeTint, 3));
  const pointMaterial = new ShaderMaterial({
    vertexShader: pointVertex,
    fragmentShader: pointFragment,
    transparent: true,
    depthWrite: false,
    uniforms: { uPixelRatio: { value: pixelRatio } },
  });
  group.add(new Points(nodeGeometry, pointMaterial));

  // Active trails and packets.
  const trailGeometry = new BufferGeometry();
  const trailPositions = new Float32Array(MAX_PACKETS * 6);
  const trailAlpha = new Float32Array(MAX_PACKETS * 2);
  const trailTint = new Float32Array(MAX_PACKETS * 6);
  trailGeometry.setAttribute("position", new BufferAttribute(trailPositions, 3));
  trailGeometry.setAttribute("alpha", new BufferAttribute(trailAlpha, 1));
  trailGeometry.setAttribute("tint", new BufferAttribute(trailTint, 3));
  group.add(new LineSegments(trailGeometry, lineMaterial));

  const packetGeometry = new BufferGeometry();
  const packetPositions = new Float32Array(MAX_PACKETS * 3);
  const packetAlpha = new Float32Array(MAX_PACKETS);
  const packetSize = new Float32Array(MAX_PACKETS);
  const packetTint = new Float32Array(MAX_PACKETS * 3);
  packetGeometry.setAttribute("position", new BufferAttribute(packetPositions, 3));
  packetGeometry.setAttribute("alpha", new BufferAttribute(packetAlpha, 1));
  packetGeometry.setAttribute("size", new BufferAttribute(packetSize, 1));
  packetGeometry.setAttribute("tint", new BufferAttribute(packetTint, 3));
  const packetMaterial = pointMaterial.clone();
  packetMaterial.blending = NormalBlending;
  group.add(new Points(packetGeometry, packetMaterial));

  function paintStatic() {
    nodes.forEach((n, i) => {
      const teammate = n.kind === "teammate";
      const c = teammate ? palette.accent : palette.faint;
      nodeTint.set([c.r, c.g, c.b], i * 3);
      spokeTint.set(
        [palette.faint.r, palette.faint.g, palette.faint.b, palette.faint.r, palette.faint.g, palette.faint.b],
        i * 6,
      );
    });
    nodeGeometry.attributes.tint.needsUpdate = true;
    spokeGeometry.attributes.tint.needsUpdate = true;
  }
  paintStatic();

  const packets: Packet[] = [];
  let lastSpawn = 0;
  let frame = 0;
  let visible = true;
  let angle = 0.6;
  let velocity = 0.0011;
  let dragging = false;
  let lastX = 0;

  function resize() {
    const { width, height } = canvas.getBoundingClientRect();
    if (!width || !height) {
      return;
    }
    renderer.setSize(width, height, false);
    camera.aspect = width / height;
    camera.updateProjectionMatrix();
  }
  resize();
  const resizeObserver = new ResizeObserver(resize);
  resizeObserver.observe(canvas);

  const intersection = new IntersectionObserver(([entry]) => {
    visible = entry.isIntersecting;
    if (visible && !frame) {
      frame = requestAnimationFrame(tick);
    }
  });
  intersection.observe(canvas);

  const themeObserver = new MutationObserver(() => {
    palette = readPalette();
    paintStatic();
  });
  themeObserver.observe(document.documentElement, { attributes: true, attributeFilter: ["class"] });

  function onDown(event: PointerEvent) {
    dragging = true;
    lastX = event.clientX;
    canvas.setPointerCapture(event.pointerId);
  }
  function onMove(event: PointerEvent) {
    if (!dragging) {
      return;
    }
    const dx = event.clientX - lastX;
    lastX = event.clientX;
    angle += dx * 0.006;
    velocity = dx * 0.0004;
  }
  function onUp() {
    dragging = false;
  }
  canvas.addEventListener("pointerdown", onDown);
  canvas.addEventListener("pointermove", onMove);
  canvas.addEventListener("pointerup", onUp);
  canvas.addEventListener("pointercancel", onUp);

  const ease = (t: number) => t * t * (3 - 2 * t);

  // biome-ignore lint/complexity/noExcessiveCognitiveComplexity: one frame of the scene reads best as one function
  function tick(now: number) {
    frame = 0;
    if (!visible) {
      return;
    }

    if (!dragging) {
      velocity += (0.0011 - velocity) * 0.02;
      angle += velocity;
    }
    group.rotation.y = angle;

    if (now - lastSpawn > SPAWN_MS && packets.length < MAX_PACKETS) {
      lastSpawn = now;
      const from = customers[Math.floor(Math.random() * customers.length)];
      packets.push({
        from,
        to: teammates[Math.floor(Math.random() * teammates.length)],
        start: now,
        phase: "in",
        routed: Math.random() < 0.32,
      });
    }

    for (let i = 0; i < nodes.length; i++) {
      glow[i] *= 0.95;
      const teammate = nodes[i].kind === "teammate";
      nodeAlpha[i] = teammate ? 0.95 : 0.55 + glow[i] * 0.45;
      nodeSize[i] = teammate ? 9 + glow[i] * 6 : 5 + glow[i] * 5;
      const c = teammate ? palette.accent : glow[i] > 0.05 ? palette.ink : palette.faint;
      nodeTint.set([c.r, c.g, c.b], i * 3);
    }

    trailAlpha.fill(0);
    packetAlpha.fill(0);
    for (let i = packets.length - 1; i >= 0; i--) {
      const p = packets[i];
      const elapsed = now - p.start;
      let point: Vector3;
      let origin: Vector3;
      let color: Color;
      if (p.phase === "in") {
        const t = Math.min(1, elapsed / IN_MS);
        origin = positions[p.from];
        point = origin.clone().multiplyScalar(1 - ease(t));
        color = palette.ink;
        if (t === 0 || elapsed < 30) {
          glow[p.from] = 1;
        }
        if (t >= 1) {
          if (p.routed) {
            p.phase = "out";
            p.start = now;
          } else {
            packets.splice(i, 1);
            options.onResolved();
          }
          continue;
        }
      } else {
        const t = Math.min(1, elapsed / OUT_MS);
        origin = new Vector3(0, 0, 0);
        point = positions[p.to].clone().multiplyScalar(ease(t));
        color = palette.accent;
        if (t >= 1) {
          glow[p.to] = 1;
          packets.splice(i, 1);
          options.onRouted();
          continue;
        }
      }
      const slot = i % MAX_PACKETS;
      packetPositions.set([point.x, point.y, point.z], slot * 3);
      packetAlpha[slot] = 1;
      packetSize[slot] = 7;
      packetTint.set([color.r, color.g, color.b], slot * 3);
      trailPositions.set([origin.x, origin.y, origin.z, point.x, point.y, point.z], slot * 6);
      trailAlpha.set([0.15, 0.85], slot * 2);
      trailTint.set([color.r, color.g, color.b, color.r, color.g, color.b], slot * 6);
    }

    for (const geometry of [nodeGeometry, trailGeometry, packetGeometry]) {
      for (const name of Object.keys(geometry.attributes)) {
        geometry.attributes[name].needsUpdate = true;
      }
    }

    renderer.render(scene, camera);
    frame = requestAnimationFrame(tick);
  }
  frame = requestAnimationFrame(tick);

  return () => {
    cancelAnimationFrame(frame);
    resizeObserver.disconnect();
    intersection.disconnect();
    themeObserver.disconnect();
    canvas.removeEventListener("pointerdown", onDown);
    canvas.removeEventListener("pointermove", onMove);
    canvas.removeEventListener("pointerup", onUp);
    canvas.removeEventListener("pointercancel", onUp);
    renderer.dispose();
  };
}
