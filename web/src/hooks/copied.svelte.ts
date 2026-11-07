import { onDestroy } from "svelte";
import { copyText } from "../clipboard";

/**
 * Copy feedback: `copied` turns true after a successful copy and resets after
 * `ms`; `failed` stays true until the next attempt succeeds.
 */
export function useCopied(ms = 2000) {
  let copied = $state(false);
  let failed = $state(false);
  let timer: ReturnType<typeof setTimeout> | undefined;
  onDestroy(() => clearTimeout(timer));

  return {
    get copied() {
      return copied;
    },
    get failed() {
      return failed;
    },
    async copy(text: string) {
      copied = await copyText(text);
      failed = !copied;
      clearTimeout(timer);
      timer = setTimeout(() => (copied = false), ms);
      return copied;
    },
  };
}
