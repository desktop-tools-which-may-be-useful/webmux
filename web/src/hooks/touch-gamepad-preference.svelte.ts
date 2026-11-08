import { store, stored } from "../api";

// Device-local preference for the floating touch gamepad overlay.
// Every mounted terminal pane shares the same reactive module state.
let enabled = $state(stored("touchGamepad") === "1");

export const touchGamepad = {
  get enabled() {
    return enabled;
  },
  set enabled(value: boolean) {
    enabled = value;
    store("touchGamepad", value ? "1" : "0");
  },
};
