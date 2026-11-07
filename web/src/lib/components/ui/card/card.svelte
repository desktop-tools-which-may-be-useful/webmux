<script lang="ts">
  import { cn, type WithElementRef } from "$lib/utils.js";
  import type { HTMLAttributes } from "svelte/elements";

  let {
    ref = $bindable(null),
    class: className,
    children,
    size = "default",
    flat = false,
    ...restProps
  }: WithElementRef<HTMLAttributes<HTMLDivElement>> & {
    size?: "default" | "sm";
    /** On a phone the card drops its frame and surface: a section, not a box. */
    flat?: boolean;
  } = $props();
</script>

<div
  bind:this={ref}
  data-slot="card"
  data-size={size}
  data-flat={flat ? "" : undefined}
  class={cn(
    "ring-foreground/10 bg-card text-card-foreground gap-(--card-spacing) overflow-hidden rounded-lg py-(--card-spacing) text-sm/relaxed ring-1 [--card-spacing:--spacing(4)] has-data-[slot=card-footer]:pb-0 has-[>img:first-child]:pt-0 data-[size=sm]:[--card-spacing:--spacing(3)] data-[size=sm]:has-data-[slot=card-footer]:pb-0 *:[img:first-child]:rounded-none *:[img:last-child]:rounded-none group/card flex flex-col",
    flat &&
      "max-sm:rounded-none max-sm:bg-transparent max-sm:border-t max-sm:ring-0 max-sm:*:data-[slot=card-content]:px-0 max-sm:*:data-[slot=card-header]:px-0",
    className,
  )}
  {...restProps}
>
  {@render children?.()}
</div>
