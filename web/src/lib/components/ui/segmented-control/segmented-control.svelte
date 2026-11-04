<script lang="ts" module>
  import { tv, type VariantProps } from "tailwind-variants";

  // A single-choice radio group drawn like the line tabs: an underline marks
  // the chosen segment instead of a filled one.
  export const segmentedControlVariants = tv({
    base: "group/segmented-control inline-flex w-fit items-center justify-center gap-1 p-[3px] text-muted-foreground",
    variants: {
      size: {
        default: "h-8",
        form: "h-[calc(var(--control-default)+6px)]",
      },
      // Segments flow onto more rows on narrow screens; each row keeps room
      // for the underline below it.
      wrap: { true: "h-auto flex-wrap justify-start gap-y-2" },
    },
    defaultVariants: { size: "default" },
  });
</script>

<script lang="ts">
  import { RadioGroup as RadioGroupPrimitive } from "bits-ui";
  import { cn } from "$lib/utils.js";

  let {
    ref = $bindable(null),
    value = $bindable(""),
    size = "default",
    wrap = false,
    class: className,
    ...restProps
  }: Omit<RadioGroupPrimitive.RootProps, "orientation"> & {
    size?: VariantProps<typeof segmentedControlVariants>["size"];
    wrap?: boolean;
  } = $props();
</script>

<RadioGroupPrimitive.Root
  bind:ref
  bind:value
  orientation="horizontal"
  data-slot="segmented-control"
  data-size={size}
  data-wrap={wrap || undefined}
  class={cn(segmentedControlVariants({ size, wrap }), className)}
  {...restProps}
/>
