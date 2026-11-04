#!/usr/bin/env bash
# Decide whether a release can reuse the complete Test run that main already
# finished for the tagged commit, instead of running the whole suite again.
#
# Usage: require-main-test.sh <commit sha>
# Writes reuse=true|false to $GITHUB_OUTPUT. It is true only when the commit is
# on main and its `push` Test run succeeded with no skipped job (main pushes
# always run the full suite). Every other case returns false, so the Release
# workflow falls back to running the suite itself; a failed main run is
# therefore never a way around the gate.
#
# Needs GH_TOKEN (actions: read), a checkout with full history and the gh CLI.
set -euo pipefail

sha=${1:?usage: require-main-test.sh <commit sha>}
appear_timeout=${APPEAR_TIMEOUT:-600}
watch_timeout=${WATCH_TIMEOUT:-2400}
output=${GITHUB_OUTPUT:-/dev/stdout}

decide() {
  echo "reuse: $2"
  echo "reuse=$1" >>"$output"
  exit 0
}

git fetch --quiet origin main
if ! git merge-base --is-ancestor "$sha" origin/main; then
  decide false "$sha is not on main"
fi

# The push run starts right after the merge; give it time to be created.
run=""
deadline=$((SECONDS + appear_timeout))
while :; do
  run=$(gh run list --workflow test.yml --branch main --event push --commit "$sha" \
    --limit 1 --json databaseId --jq '.[0].databaseId // empty')
  [ -n "$run" ] && break
  [ "$SECONDS" -ge "$deadline" ] && decide false "no Test run on main for $sha"
  sleep 15
done

# A queued or slow run must not time out the whole job: give up waiting and let
# `checks` run the suite instead. `timeout` exits 124 when the budget is spent;
# any other failure (a failed run) still falls through to the result check.
echo "waiting for Test run $run on main (up to ${watch_timeout}s)"
status=0
timeout "$watch_timeout" gh run watch "$run" --interval 30 >/dev/null 2>&1 ||
  status=$?
if [ "$status" -eq 124 ]; then
  decide false "Test run $run did not finish within ${watch_timeout}s"
fi

result=$(gh run view "$run" --json conclusion,jobs --jq '
  if .conclusion != "success" then "run concluded \(.conclusion)"
  elif ([.jobs[] | select(.conclusion == "skipped")] | length) > 0 then "run skipped jobs"
  elif ([.jobs[] | select(.name == "Test result" and .conclusion == "success")] | length) != 1 then "no passing Test result"
  else "ok" end')
if [ "$result" != ok ]; then
  decide false "Test run $run is not a complete pass ($result)"
fi
decide true "Test run $run passed in full for $sha"
