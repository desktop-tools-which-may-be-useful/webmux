// Rows for the start-commit dropdown. A row shows the commit title first and
// its short hash, tip tags and age second, so users recognise a commit by what
// it did and how recent it is instead of typing a hash.
import type { z } from "zod";
import type { CommitsSchema } from "./schemas";

export type Commit = z.infer<typeof CommitsSchema>["commits"][number];
/** A branch tip the history was read from, e.g. the local or upstream tip. */
export type CommitTip = { hash: string; tag: string };
export type CommitChoice = {
  value: string;
  subject: string;
  shortHash: string;
  tags: string[];
  age: string;
};

const units: Array<[Intl.RelativeTimeFormatUnit, number]> = [
  ["year", 365 * 24 * 3600],
  ["month", 30 * 24 * 3600],
  ["week", 7 * 24 * 3600],
  ["day", 24 * 3600],
  ["hour", 3600],
  ["minute", 60],
];
const formatter = new Intl.RelativeTimeFormat("zh-CN", { numeric: "auto" });

export function relativeTime(iso: string, now = Date.now()): string {
  const seconds = (Date.parse(iso) - now) / 1000;
  if (Number.isNaN(seconds)) return "";
  for (const [unit, size] of units) {
    if (Math.abs(seconds) >= size)
      return formatter.format(Math.trunc(seconds / size), unit);
  }
  return "刚刚";
}

const subjectOf = (subject: string) => subject || "无标题提交";

/** Text naming a row for screen readers, typeahead and tests. */
export function commitChoiceLabel(choice: CommitChoice): string {
  return [
    choice.subject,
    choice.shortHash,
    ...choice.tags,
    ...(choice.age ? [choice.age] : []),
  ].join(" · ");
}

/**
 * Dropdown rows for `commits` (newest first). Each commit that is one of the
 * `tips` carries that tip's tag. A selected `base` outside the list (for
 * example while the history is still loading) stays selectable as its own
 * leading row.
 */
export function commitChoices(
  commits: Commit[],
  options: {
    tips: CommitTip[];
    base: string;
    baseSubject: string;
    now?: number;
  },
): CommitChoice[] {
  const tagsOf = (hash: string) =>
    options.tips.filter((tip) => tip.hash === hash).map((tip) => tip.tag);
  const rows = commits.map((commit) => ({
    value: commit.hash,
    subject: subjectOf(commit.subject),
    shortHash: commit.hash.slice(0, 7),
    tags: tagsOf(commit.hash),
    age: relativeTime(commit.committedAt, options.now),
  }));
  if (!options.base || rows.some((row) => row.value === options.base))
    return rows;
  return [
    {
      value: options.base,
      subject: subjectOf(options.baseSubject),
      shortHash: options.base.slice(0, 7),
      tags: tagsOf(options.base),
      age: "",
    },
    ...rows,
  ];
}
