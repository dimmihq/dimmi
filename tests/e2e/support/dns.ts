/*
 * A DNS server for the end-to-end tests, so domain checks read records a
 * test publishes instead of the internet. Each name's records live in a
 * JSON file of its own in one folder, so tests running side by side
 * never overwrite each other; a name without a file does not exist. It
 * answers TXT, MX and NS questions over UDP, which is all a sending
 * domain check asks for.
 */
import { createSocket } from "node:dgram";
import { mkdirSync, readFileSync, renameSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

export interface ZoneRecords {
  TXT?: string[];
  MX?: { host: string; pref: number }[];
  NS?: string[];
}

const types = { TXT: 16, MX: 15, NS: 2 } as const;

/*
 * zoneFolder is where the records of the server for the port live.
 */
export function zoneFolder(port: number | string): string {
  return join(tmpdir(), `dimmi-e2e-dns-${port}`);
}

/*
 * publish puts the records at the name, replacing what was there.
 */
export function publish(port: number | string, name: string, records: ZoneRecords) {
  const folder = zoneFolder(port);
  mkdirSync(folder, { recursive: true });
  const file = join(folder, `${name.toLowerCase()}.json`);
  writeFileSync(`${file}.tmp`, JSON.stringify(records));
  renameSync(`${file}.tmp`, file);
}

function readName(message: Buffer, start: number): [string, number] {
  const labels: string[] = [];
  let at = start;
  while (message[at] !== 0 && at < message.length) {
    const size = message[at] ?? 0;
    labels.push(message.subarray(at + 1, at + 1 + size).toString("latin1"));
    at += size + 1;
  }
  return [labels.join(".").toLowerCase(), at + 1];
}

function encodeName(name: string): Buffer {
  const parts = name.split(".").filter(Boolean);
  return Buffer.concat([
    ...parts.map((label) => Buffer.concat([Buffer.from([label.length]), Buffer.from(label, "latin1")])),
    Buffer.from([0]),
  ]);
}

function txtData(value: string): Buffer {
  const chunks: Buffer[] = [];
  for (let at = 0; at === 0 || at < value.length; at += 255) {
    const piece = Buffer.from(value.slice(at, at + 255), "latin1");
    chunks.push(Buffer.concat([Buffer.from([piece.length]), piece]));
  }
  return Buffer.concat(chunks);
}

function answers(records: ZoneRecords, type: number): Buffer[] {
  if (type === types.TXT) {
    return (records.TXT ?? []).map(txtData);
  }
  if (type === types.MX) {
    return (records.MX ?? []).map((mx) => {
      const pref = Buffer.alloc(2);
      pref.writeUInt16BE(mx.pref);
      return Buffer.concat([pref, encodeName(mx.host)]);
    });
  }
  if (type === types.NS) {
    return (records.NS ?? []).map(encodeName);
  }
  return [];
}

function lookup(folder: string, name: string): ZoneRecords | null {
  try {
    return JSON.parse(readFileSync(join(folder, `${name}.json`), "utf8")) as ZoneRecords;
  } catch {
    return null;
  }
}

/*
 * respond builds the answer to one query from the folder's records.
 */
function respond(folder: string, query: Buffer): Buffer {
  const [name, end] = readName(query, 12);
  const type = query.readUInt16BE(end);
  const records = lookup(folder, name);
  const data = records ? answers(records, type) : [];

  const header = Buffer.from(query.subarray(0, 12));
  header.writeUInt16BE(0x8400 | (query.readUInt16BE(2) & 0x0100) | (records ? 0 : 3), 2);
  header.writeUInt16BE(1, 4);
  header.writeUInt16BE(data.length, 6);
  header.writeUInt16BE(0, 8);
  header.writeUInt16BE(0, 10);

  const rows = data.map((rdata) => {
    const fixed = Buffer.alloc(12);
    fixed.writeUInt16BE(0xc00c, 0);
    fixed.writeUInt16BE(type, 2);
    fixed.writeUInt16BE(1, 4);
    fixed.writeUInt32BE(1, 6);
    fixed.writeUInt16BE(rdata.length, 10);
    return Buffer.concat([fixed, rdata]);
  });

  return Buffer.concat([header, query.subarray(12, end + 4), ...rows]);
}

/*
 * serveZone answers queries on a free local port from the folder of the
 * server port, and returns the address to point resolvers at.
 */
export function serveZone(serverPort: number): Promise<string> {
  const folder = zoneFolder(serverPort);
  mkdirSync(folder, { recursive: true });
  const socket = createSocket("udp4");
  socket.on("message", (message, from) => {
    try {
      socket.send(respond(folder, message), from.port, from.address);
    } catch {
      return;
    }
  });
  socket.unref();

  return new Promise((resolveAddress) => {
    socket.bind(0, "127.0.0.1", () => resolveAddress(`127.0.0.1:${socket.address().port}`));
  });
}
