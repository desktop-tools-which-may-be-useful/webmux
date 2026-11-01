// A resize "redraw storm" in the style of omp-like agent CLIs: on every size
// change the program clears the screen and scrollback, deletes its images and
// reprints the whole transcript, wrapped to the new width, with text and Kitty
// PNG images interleaved. The same bytes drive the interactive PTY program
// (redraw-storm-demo.mjs), the performance spec and manual reproduction.
import { crc32, deflateSync } from "node:zlib";

export interface StormOptions {
  cols: number;
  rows: number;
  /** Increments on every redraw so each storm's final screen is distinct. */
  generation: number;
  /** Transcript entries; every entry is several wrapped lines of text. */
  messages?: number;
  /** Every Nth entry carries an image (0 disables images). */
  imageEvery?: number;
  imagePixels?: { width: number; height: number };
  imageCells?: { columns: number; rows: number };
  /** Wrap the redraw in DEC 2026 synchronized output, as modern TUIs do. */
  sync?: boolean;
}

const ESC = "\x1b";
const kittyChunk = 4096;

function random(seed: number) {
  let state = seed >>> 0 || 1;
  return () => {
    state ^= state << 13;
    state ^= state >>> 17;
    state ^= state << 5;
    return (state >>> 0) / 0x1_0000_0000;
  };
}

function pngChunk(type: string, data: Buffer) {
  const length = Buffer.alloc(4);
  length.writeUInt32BE(data.length);
  const body = Buffer.concat([Buffer.from(type, "latin1"), data]);
  const checksum = Buffer.alloc(4);
  checksum.writeUInt32BE(crc32(body) >>> 0);
  return Buffer.concat([length, body, checksum]);
}

/** A deterministic RGB PNG: a gradient with noise, so it barely compresses. */
function stormPng(index: number, width = 160, height = 96): Buffer {
  const next = random(0x9e37 + index * 7919);
  const raw = Buffer.alloc((width * 3 + 1) * height);
  for (let y = 0; y < height; y++) {
    const row = y * (width * 3 + 1);
    raw[row] = 0;
    for (let x = 0; x < width; x++) {
      const offset = row + 1 + x * 3;
      const noise = next() * 96;
      raw[offset] = (x * 255) / width / 2 + noise;
      raw[offset + 1] = (y * 255) / height / 2 + noise;
      raw[offset + 2] = ((index * 47) % 128) + noise;
    }
  }
  const header = Buffer.alloc(13);
  header.writeUInt32BE(width, 0);
  header.writeUInt32BE(height, 4);
  header[8] = 8; // bit depth
  header[9] = 2; // RGB
  return Buffer.concat([
    Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]),
    pngChunk("IHDR", header),
    pngChunk("IDAT", deflateSync(raw)),
    pngChunk("IEND", Buffer.alloc(0)),
  ]);
}

const words = [
  "render",
  "session",
  "terminal",
  "resize",
  "buffer",
  "frame",
  "终端",
  "重新绘制",
  "图片",
  "scrollback",
  "kitty",
  "graphics",
  "agent",
  "工具调用",
  "diff",
  "patch",
  "latency",
  "throughput",
];

function width(text: string) {
  let cells = 0;
  for (const char of text) cells += char.codePointAt(0)! > 0x2e80 ? 2 : 1;
  return cells;
}

function paragraph(next: () => number, cols: number, length: number) {
  const lines: string[] = [];
  let line = "";
  let used = 0;
  const limit = Math.max(8, cols - 4);
  for (let i = 0; i < length; i++) {
    const word = words[Math.floor(next() * words.length)]!;
    const cells = width(word);
    if (used + cells + 1 > limit) {
      lines.push(line);
      line = "";
      used = 0;
    }
    const color = 31 + Math.floor(next() * 6);
    line += `${used ? " " : ""}${next() < 0.3 ? `${ESC}[${color}m${word}${ESC}[39m` : word}`;
    used += cells + (used ? 1 : 0);
  }
  if (line) lines.push(line);
  return lines;
}

/** The writes of one image: reserve rows, then place it above the cursor. */
function imageWrites(
  id: number,
  png: Buffer,
  cells: { columns: number; rows: number },
) {
  const data = png.toString("base64");
  // Scroll real lines into existence first: CUD does not scroll, so this is
  // how omp keeps an image near the bottom from overlapping later text.
  const writes = ["\r\n".repeat(cells.rows)];
  writes.push(`${ESC}7${ESC}[${cells.rows}A`);
  for (let start = 0; start < data.length; start += kittyChunk) {
    const more = start + kittyChunk < data.length ? 1 : 0;
    const control =
      start === 0
        ? `a=T,f=100,i=${id},c=${cells.columns},r=${cells.rows},C=1,q=2,m=${more}`
        : `m=${more}`;
    writes.push(
      `${ESC}_G${control};${data.slice(start, start + kittyChunk)}${ESC}\\`,
    );
  }
  writes.push(`${ESC}8`);
  return writes;
}

const stormMarker = (generation: number, cols: number, rows: number) =>
  `storm ${generation} ${cols}x${rows} done`;

/**
 * One complete redraw as the sequence of separate write(2) calls a TUI makes:
 * one per text line and one per Kitty chunk, so the PTY sees many small writes.
 */
export function stormWrites(options: StormOptions): string[] {
  const {
    cols,
    rows,
    generation,
    messages = 60,
    imageEvery = 5,
    imagePixels = { width: 160, height: 96 },
    imageCells = { columns: 20, rows: 6 },
    sync = true,
  } = options;
  const next = random(generation * 31 + cols);
  const writes: string[] = [];
  if (sync) writes.push(`${ESC}[?2026h`);
  // Clear screen and scrollback, drop every image, then reprint everything.
  writes.push(
    `${ESC}[0m${ESC}[2J${ESC}[3J${ESC}[H`,
    `${ESC}_Ga=d,d=A,q=2${ESC}\\`,
  );
  for (let index = 0; index < messages; index++) {
    const role = index % 2 ? "assistant" : "user";
    writes.push(
      `${ESC}[1;${index % 2 ? 36 : 33}m● ${role} #${index}${ESC}[0m ${ESC}[2m(${cols}x${rows})${ESC}[0m\r\n`,
    );
    for (const line of paragraph(next, cols, 18 + Math.floor(next() * 30)))
      writes.push(`  ${line}${ESC}[0m\r\n`);
    if (imageEvery > 0 && index % imageEvery === imageEvery - 1) {
      const image = (index + 1) / imageEvery;
      writes.push(
        ...imageWrites(
          image,
          stormPng(image, imagePixels.width, imagePixels.height),
          imageCells,
        ),
      );
    }
    if (index % 3 === 2)
      writes.push(
        `${ESC}[48;5;236m  $ go test ./internal/hub ${" ".repeat(Math.max(0, cols - 28))}${ESC}[0m\r\n`,
      );
  }
  writes.push(`${ESC}[7m${stormMarker(generation, cols, rows)}${ESC}[0m`);
  if (sync) writes.push(`${ESC}[?2026l`);
  return writes;
}
