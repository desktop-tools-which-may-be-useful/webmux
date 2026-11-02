import type { ChatConfig } from "./schemas";

// The labels an option's choices are shown with. OMP names its models by
// display name ("Claude Haiku 4.5"), which several providers and snapshots
// share; the value is the full "provider/model" id, so that is what shows.
export function choicesOf(
  agent: string,
  option: ChatConfig,
): { value: string; name: string }[] {
  if (agent !== "omp" || option.category !== "model") return option.options;
  return option.options.map((choice) => ({
    value: choice.value,
    name: choice.value,
  }));
}
