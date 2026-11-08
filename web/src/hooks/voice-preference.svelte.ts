import { store, stored } from "../api";

// Whether this browser offers the microphone. Microphone access and the
// wish to dictate differ per device, so this is a per-browser preference and
// never syncs through server settings; it only matters once the server has
// speech recognition configured. On unless the user turned it off.
const key = "voiceInput";

let enabled = $state(stored(key, "1") !== "0");

export const voicePreference = {
  get enabled() {
    return enabled;
  },
  set(value: boolean) {
    enabled = value;
    store(key, value ? "1" : "0");
  },
};
