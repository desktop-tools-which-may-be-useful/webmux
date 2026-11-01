// Run in a WebMux session, then resize the browser window:
//   node web/test/fixtures/redraw-storm-demo.mjs [--messages 60] [--image-every 5] [--no-sync]
// Every size change reprints the whole transcript, with text and Kitty images
// mixed, as one burst of small writes. r redraws; q or Ctrl-C quits.
import { parseArgs } from "node:util";
import { stormWrites } from "./redraw-storm.ts";

const { values } = parseArgs({
  options: {
    messages: { type: "string", default: "60" },
    "image-every": { type: "string", default: "5" },
    "no-sync": { type: "boolean", default: false },
  },
});
const options = {
  messages: Number(values.messages),
  imageEvery: Number(values["image-every"]),
  sync: !values["no-sync"],
};

let generation = 0;
function draw() {
  generation++;
  const writes = stormWrites({
    ...options,
    cols: process.stdout.columns,
    rows: process.stdout.rows,
    generation,
  });
  // Writes to a TTY are synchronous on Linux and macOS: one write(2) each.
  for (const data of writes) process.stdout.write(data);
}

process.stdin.setRawMode(true);
process.stdin.resume();
process.stdout.on("resize", draw);
process.stdin.on("data", (data) => {
  const key = data.toString();
  if (key.includes("q") || key.includes("\x03")) {
    process.stdout.write("\x1b_Ga=d,d=A,q=2\x1b\\\x1b[0m\x1b[2J\x1b[3J\x1b[H");
    process.stdin.setRawMode(false);
    process.exit(0);
  }
  if (key.includes("r")) draw();
});
draw();
