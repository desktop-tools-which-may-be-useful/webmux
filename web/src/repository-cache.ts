import type { Repository } from "./schemas";

// The last successful Git discovery, kept across route changes so returning
// from a terminal renders the previous grouping before a background refresh.
let last: Repository[] | undefined;

export function lastRepositories(): Repository[] | undefined {
  return last;
}

export function rememberRepositories(repositories: Repository[]): void {
  last = repositories;
}

export function clearRepositories(): void {
  last = undefined;
}
