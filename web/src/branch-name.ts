// Suggested names are a form convenience. Creation still sends an explicit branch;
// the server keeps check-ref-format and the 409 conflict as the authority.
const ADJECTIVES = [
  "amber",
  "brisk",
  "calm",
  "clear",
  "crisp",
  "eager",
  "fair",
  "gentle",
  "keen",
  "kind",
  "lively",
  "lucid",
  "mild",
  "nimble",
  "quiet",
  "rapid",
  "steady",
  "sunny",
  "swift",
  "tidy",
  "vivid",
  "warm",
  "bright",
  "mellow",
] as const;

const NOUNS = [
  "brook",
  "cedar",
  "cliff",
  "comet",
  "creek",
  "delta",
  "fern",
  "field",
  "finch",
  "grove",
  "harbor",
  "heron",
  "maple",
  "meadow",
  "otter",
  "pine",
  "ridge",
  "river",
  "sparrow",
  "stone",
  "summit",
  "trail",
  "willow",
  "wren",
] as const;

export const worktreeBranchAdjectives = ADJECTIVES;
export const worktreeBranchNouns = NOUNS;

const PAIRS: ReadonlyArray<readonly [string, string]> = ADJECTIVES.flatMap(
  (adjective) => NOUNS.map((noun) => [adjective, noun] as const),
);
const MAX_PAIR_SUFFIX = 20;

export interface BranchNameInput {
  branchNames: readonly string[];
  worktreePaths: readonly string[];
  avoid?: string;
}

export interface BranchNameOptions {
  random?: () => number;
  now?: Date;
}

function shuffled<T>(items: readonly T[], random: () => number): T[] {
  const copy = [...items];
  for (let index = copy.length - 1; index > 0; index--) {
    const raw = random();
    const scaled = Math.floor((Number.isFinite(raw) ? raw : 0) * (index + 1));
    const swap = scaled <= 0 ? 0 : scaled > index ? index : scaled;
    [copy[index], copy[swap]] = [copy[swap]!, copy[index]!];
  }
  return copy;
}

function free(
  name: string,
  segment: string,
  refs: ReadonlySet<string>,
  segments: ReadonlySet<string>,
  avoid?: string,
): boolean {
  return name !== avoid && !refs.has(name) && !segments.has(segment);
}

function timestampBranch(
  now: Date,
  refs: ReadonlySet<string>,
  segments: ReadonlySet<string>,
  avoid?: string,
): string {
  const stamp = `${now.getFullYear()}${String(now.getMonth() + 1).padStart(2, "0")}${String(now.getDate()).padStart(2, "0")}-${String(now.getHours()).padStart(2, "0")}${String(now.getMinutes()).padStart(2, "0")}${String(now.getSeconds()).padStart(2, "0")}`;
  for (let suffix = 1; suffix <= 1000; suffix++) {
    const segment = suffix === 1 ? stamp : `${stamp}-${suffix}`;
    const name = `wt/${segment}`;
    if (free(name, segment, refs, segments, avoid)) return name;
  }
  return `wt/${stamp}-${now.getMilliseconds()}`;
}

export function suggestWorktreeBranch(
  input: BranchNameInput,
  options: BranchNameOptions = {},
): string {
  const random = options.random ?? Math.random;
  const refs = new Set(input.branchNames);
  const segments = new Set(
    input.worktreePaths
      .map(
        (path) =>
          path
            .split("/")
            .filter((part) => part !== "")
            .at(-1) ?? "",
      )
      .filter((segment) => segment !== ""),
  );
  const ordered = shuffled(PAIRS, random);
  for (const [adjective, noun] of ordered) {
    const segment = `${adjective}-${noun}`;
    const name = `wt/${segment}`;
    if (free(name, segment, refs, segments, input.avoid)) return name;
  }
  for (const [adjective, noun] of ordered) {
    for (let suffix = 2; suffix <= MAX_PAIR_SUFFIX; suffix++) {
      const segment = `${adjective}-${noun}-${suffix}`;
      const name = `wt/${segment}`;
      if (free(name, segment, refs, segments, input.avoid)) return name;
    }
  }
  return timestampBranch(
    options.now ?? new Date(),
    refs,
    segments,
    input.avoid,
  );
}

const REMOTE_REF_PREFIX = "refs/remotes/";

export interface RemoteBranchBase {
  name: string;
  head: string;
  subject: string;
}

/**
 * Extracts remote-tracking branches (`origin/main`, `origin/feature/x`, etc.)
 * from `/api/repositories/{id}/branches` bases, skipping symbolic `/HEAD` refs
 * and any short name that collides with an existing local branch name.
 */
export function remoteBranchBases(
  bases: readonly RemoteBranchBase[],
  localNames: Iterable<string> = [],
): RemoteBranchBase[] {
  const locals = new Set(localNames);
  const result: RemoteBranchBase[] = [];
  for (const base of bases) {
    if (
      !base.name.startsWith(REMOTE_REF_PREFIX) ||
      base.name.endsWith("/HEAD")
    ) {
      continue;
    }
    const short = base.name.slice(REMOTE_REF_PREFIX.length);
    if (!short || !short.includes("/") || locals.has(short)) continue;
    result.push({ name: short, head: base.head, subject: base.subject });
  }
  return result;
}
