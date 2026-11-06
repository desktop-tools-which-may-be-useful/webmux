#!/usr/bin/env bash
set -euo pipefail

# Filters a Go coverage profile to exclude test-only guard and fixture packages,
# writes an HTML report and a Markdown summary table (printing to stdout and
# appending to $GITHUB_STEP_SUMMARY when running in GitHub Actions).

raw_profile="${1:?usage: go-coverage-summary.sh <raw.coverprofile> <output-dir>}"
out_dir="${2:?usage: go-coverage-summary.sh <raw.coverprofile> <output-dir>}"

mkdir -p "$out_dir"
filtered_profile="$out_dir/go.coverprofile"
html_report="$out_dir/go.html"
summary_md="$out_dir/summary.md"

# Exclude pure test-guard/fixture/benchmark packages from production metrics.
grep -v -E '^github\.com/rainchan/webmux/(internal/archtest|internal/docguard|internal/agentprofile/agenttest|internal/acp/testdata/fakeagent|cmd/baseline)/' \
  "$raw_profile" > "$filtered_profile"

go tool cover -html="$filtered_profile" -o "$html_report"
total_pct="$(go tool cover -func="$filtered_profile" | awk '/^total:/ {print $3}')"

awk '
NR == 1 { next }
{
  split($1, loc, ":")
  file = loc[1]
  sub(/^github\.com\/rainchan\/webmux\//, "", file)
  n = split(file, parts, "/")
  pkg = parts[1]
  for (i = 2; i < n; i++) {
    pkg = pkg "/" parts[i]
  }
  stmts = $2
  count = $3
  key = $1
  if (!(key in block_stmts)) {
    block_stmts[key] = stmts
    block_pkg[key] = pkg
    block_hit[key] = 0
  }
  if (count > 0) {
    block_hit[key] = 1
  }
}
END {
  total_s = 0
  hit_s = 0
  for (k in block_stmts) {
    p = block_pkg[k]
    s = block_stmts[k]
    pkg_total[p] += s
    total_s += s
    if (block_hit[k]) {
      pkg_hit[p] += s
      hit_s += s
    }
  }
  total_pct = (total_s > 0) ? (hit_s * 100.0 / total_s) : 0
  printf "### Go Coverage\n\n"
  printf "| Package | Coverage | Statements |\n"
  printf "| :--- | ---: | ---: |\n"
  printf "| **Total** | **%.1f%%** | **%d / %d** |\n", total_pct, hit_s, total_s
  for (p in pkg_total) {
    pct = (pkg_total[p] > 0) ? (pkg_hit[p] * 100.0 / pkg_total[p]) : 0
    printf "| `%s` | %.1f%% | %d / %d |\n", p, pct, pkg_hit[p] + 0, pkg_total[p]
  }
}
' "$filtered_profile" | {
  IFS= read -r line1
  IFS= read -r line2
  IFS= read -r line3
  IFS= read -r line4
  IFS= read -r line5
  printf '%s\n%s\n%s\n%s\n%s\n' "$line1" "$line2" "$line3" "$line4" "$line5"
  sort
} > "$summary_md"

cat "$summary_md"
echo ""
echo "Go HTML report: $html_report (func total: $total_pct)"

if [ -n "${GITHUB_STEP_SUMMARY:-}" ]; then
  {
    cat "$summary_md"
    echo ""
  } >> "$GITHUB_STEP_SUMMARY"
fi
