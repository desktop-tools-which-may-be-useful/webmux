import { onMount } from "svelte";
import type { z } from "zod";
import { api } from "../api";
import { errorMessage } from "../error";

/** One request issued through {@link LatestRequest}. */
export interface RequestTicket {
  signal: AbortSignal;
  /** The owner is alive and no newer request replaced this one. */
  current(): boolean;
  /** A failure of this request should still be shown to the user. */
  reportable(): boolean;
}

/**
 * Serializes a page's requests: each new request aborts the one it replaces,
 * and a ticket lets late replies of superseded requests be dropped.
 */
export class LatestRequest {
  #version = 0;
  #inflight: AbortController | undefined;
  #disposed = false;

  get disposed() {
    return this.#disposed;
  }

  begin(): RequestTicket {
    const version = ++this.#version;
    this.#inflight?.abort();
    const controller = (this.#inflight = new AbortController());
    const current = () => !this.#disposed && version === this.#version;
    return {
      signal: controller.signal,
      current,
      reportable: () => current() && !controller.signal.aborted,
    };
  }

  dispose() {
    this.#disposed = true;
    this.#version++;
    this.#inflight?.abort();
  }
}

/** Reads path once after mount; the request is aborted on unmount. */
export function useMountedRead<T>(path: string, schema: z.ZodType<T>) {
  const state = $state<{ value: T | undefined; error: string }>({
    value: undefined,
    error: "",
  });
  onMount(() => {
    const controller = new AbortController();
    void api(path, schema, "GET", undefined, controller.signal)
      .then((value) => {
        state.value = value;
      })
      .catch((cause: unknown) => {
        if (!controller.signal.aborted) state.error = errorMessage(cause);
      });
    return () => controller.abort();
  });
  return state;
}
