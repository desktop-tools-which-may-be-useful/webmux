<script lang="ts">
  import { RadioGroup as RadioGroupPrimitive } from "bits-ui";
  import { cn } from "$lib/utils.js";

  // An option that explains itself: a radio mark, a title and a description.
  // The border, not a fill, shows the chosen option. Pass `aria-label` with
  // the title so the description stays out of the accessible name.
  let {
    ref = $bindable(null),
    title,
    description,
    class: className,
    ...restProps
  }: Omit<RadioGroupPrimitive.ItemProps, "children"> & {
    title: string;
    description?: string;
  } = $props();
</script>

<RadioGroupPrimitive.Item
  bind:ref
  data-slot="radio-group-item"
  class={cn(
    "group/radio-item flex w-full items-start gap-3 rounded-md border border-border px-3 py-2.5 text-left transition-colors outline-none hover:bg-muted/50 focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 disabled:pointer-events-none disabled:opacity-50 data-[state=checked]:border-foreground",
    className,
  )}
  {...restProps}
>
  <span
    aria-hidden="true"
    class="mt-0.5 flex size-4 shrink-0 items-center justify-center rounded-full border border-input group-data-[state=checked]/radio-item:border-foreground"
  >
    <span
      class="size-2 rounded-full bg-foreground opacity-0 group-data-[state=checked]/radio-item:opacity-100"
    ></span>
  </span>
  <span class="grid min-w-0 gap-0.5">
    <span class="text-sm font-medium">{title}</span>
    {#if description}<span class="text-xs text-muted-foreground"
        >{description}</span
      >{/if}
  </span>
</RadioGroupPrimitive.Item>
