import { readdir, readFile, stat, writeFile } from "node:fs/promises";
import { resolve, join } from "node:path";
import { promisify } from "node:util";
import { brotliCompress, constants, gzip } from "node:zlib";
import { zipSync, type Zippable } from "fflate";
import type { Plugin } from "vite";

const brotli = promisify(brotliCompress);
const gzipAsync = promisify(gzip);
// Mirrors compressibleStaticType in internal/api/static.go. Matching files of
// at least 1 KiB are embedded only as stored .br and .gz siblings, so serving
// spends no CPU; the server inflates .gz for the rare identity request.
const compressible = /\.(html|js|mjs|css|webmanifest|svg|txt|wasm)$/;
const stored = { level: 0, mtime: new Date(1980, 0, 1) } as const;

// Keep the ordinary dist tree usable by preview servers. Go embeds only this
// archive, whose paths retain the existing fs.Sub(Assets, "dist") contract.
export function embedAssets(): Plugin {
  let outDir: string;
  return {
    name: "webmux-embed-assets",
    apply: "build",
    configResolved(config) {
      outDir = resolve(config.root, config.build.outDir);
    },
    async writeBundle() {
      const files: Zippable = {};
      const names = (await readdir(outDir, { recursive: true })).sort();
      const pending: Promise<void>[] = [];
      for (const name of names) {
        if (name === "embed.zip" || name.endsWith(".gitkeep")) continue;
        const path = join(outDir, name);
        // readdir also lists directories; only regular files are archived.
        if (!(await stat(path)).isFile()) continue;
        const entry = `dist/${name.replaceAll("\\", "/")}`;
        const data = await readFile(path);
        if (compressible.test(name) && data.length >= 1024) {
          // zlib runs these on the libuv pool, so assets compress in parallel.
          pending.push(
            (async () => {
              const [br, gz] = await Promise.all([
                brotli(data, {
                  params: {
                    [constants.BROTLI_PARAM_QUALITY]: 11,
                    [constants.BROTLI_PARAM_SIZE_HINT]: data.length,
                  },
                }),
                gzipAsync(data, { level: 9 }),
              ]);
              files[`${entry}.br`] = [br, stored];
              files[`${entry}.gz`] = [gz, stored];
            })(),
          );
          continue;
        }
        files[entry] = [
          data,
          {
            // WOFF2 and PNG are already compressed. Store them without an
            // additional decompression step on the first font request.
            level: /\.(woff2|png)$/.test(name) ? 0 : 9,
            mtime: stored.mtime,
          },
        ];
      }
      await Promise.all(pending);
      // Insertion order varies with compression timing; keep the archive
      // byte-for-byte reproducible.
      const ordered: Zippable = {};
      for (const key of Object.keys(files).sort()) ordered[key] = files[key]!;
      await writeFile(join(outDir, "embed.zip"), zipSync(ordered));
    },
  };
}
