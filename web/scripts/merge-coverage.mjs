// Merges Vitest Node unit coverage and Chromium browser coverage into one
// report under web/.vitest/coverage/merged/ (HTML, JSON, JSON summary, and a
// Markdown summary table for local output and $GITHUB_STEP_SUMMARY).
import {
  appendFileSync,
  existsSync,
  mkdirSync,
  readFileSync,
  realpathSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { createRequire } from "node:module";
import { dirname, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const webRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const coverageRoot = resolve(webRoot, ".vitest", "coverage");
const mergedDir = resolve(coverageRoot, "merged");
const inputs = [
  { name: "unit", file: resolve(coverageRoot, "unit", "coverage-final.json") },
  {
    name: "browser",
    file: resolve(coverageRoot, "browser", "coverage-final.json"),
  },
];

const available = inputs.filter((item) => existsSync(item.file));
if (available.length === 0) {
  console.error("No Vitest coverage inputs found under", coverageRoot);
  process.exit(1);
}

const providerRequire = createRequire(
  realpathSync(fileURLToPath(import.meta.resolve("@vitest/coverage-v8"))),
);
const libCoverage = await import(
  providerRequire.resolve("@vitest/istanbul-lib-coverage")
);
const libReport = await import(
  providerRequire.resolve("@vitest/istanbul-lib-report")
);

const coverageMap = libCoverage.createCoverageMap({});
for (const item of available) {
  const raw = JSON.parse(readFileSync(item.file, "utf8"));
  coverageMap.merge(raw);
}

rmSync(mergedDir, { recursive: true, force: true });
mkdirSync(mergedDir, { recursive: true });

const context = libReport.createContext({
  dir: mergedDir,
  coverageMap,
});

for (const reporterName of ["text-summary", "json", "json-summary", "html"]) {
  const report = await libReport.createAsync(reporterName, {
    projectRoot: webRoot,
  });
  report.execute(context);
}

const total = coverageMap.getCoverageSummary().toJSON();
const groups = new Map([
  ["src/*.ts (domain & state)", libCoverage.createCoverageSummary()],
  ["src/hooks/** (hooks)", libCoverage.createCoverageSummary()],
  ["src/components/** (components)", libCoverage.createCoverageSummary()],
  ["src/pages/** (pages)", libCoverage.createCoverageSummary()],
  ["src/lib/** (utilities)", libCoverage.createCoverageSummary()],
]);

for (const file of coverageMap.files().sort()) {
  const rel = relative(webRoot, file).replaceAll("\\", "/");
  const fileSummary = coverageMap.fileCoverageFor(file).toSummary();
  if (rel.startsWith("src/hooks/")) {
    groups.get("src/hooks/** (hooks)").merge(fileSummary);
  } else if (rel.startsWith("src/components/")) {
    groups.get("src/components/** (components)").merge(fileSummary);
  } else if (rel.startsWith("src/pages/")) {
    groups.get("src/pages/** (pages)").merge(fileSummary);
  } else if (rel.startsWith("src/lib/")) {
    groups.get("src/lib/** (utilities)").merge(fileSummary);
  } else if (rel.startsWith("src/")) {
    groups.get("src/*.ts (domain & state)").merge(fileSummary);
  }
}

function fmtMetric(metric) {
  return `${metric.pct.toFixed(2)}% (${metric.covered}/${metric.total})`;
}

const mdLines = [
  "### Frontend Coverage (Vitest Unit + Browser)",
  "",
  "| Scope | Statements | Branches | Functions | Lines |",
  "| :--- | ---: | ---: | ---: | ---: |",
  `| **Total** | **${fmtMetric(total.statements)}** | **${fmtMetric(total.branches)}** | **${fmtMetric(total.functions)}** | **${fmtMetric(total.lines)}** |`,
];

for (const [label, summary] of groups.entries()) {
  const data = summary.toJSON();
  if (data.statements.total === 0) continue;
  mdLines.push(
    `| \`${label}\` | ${fmtMetric(data.statements)} | ${fmtMetric(data.branches)} | ${fmtMetric(data.functions)} | ${fmtMetric(data.lines)} |`,
  );
}
mdLines.push("");

const markdown = mdLines.join("\n");
writeFileSync(resolve(mergedDir, "summary.md"), markdown);
console.log(`\n${markdown}`);
console.log(`Merged HTML report: ${resolve(mergedDir, "index.html")}`);

if (process.env.GITHUB_STEP_SUMMARY) {
  appendFileSync(process.env.GITHUB_STEP_SUMMARY, `${markdown}\n`);
}
