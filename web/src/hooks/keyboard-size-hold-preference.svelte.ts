import { store, stored } from "../api";

// The preference is device-wide, so every mounted pane reads and writes the
// same module state instead of loading its own copy from localStorage.
let enabled = $state(stored("keyboardSizeHold") !== "0");

export const keyboardSizeHold = {
  get enabled() {
    return enabled;
  },
  set enabled(value: boolean) {
    enabled = value;
    store("keyboardSizeHold", value ? "1" : "0");
  },
};
