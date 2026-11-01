import { describe, expect, test } from "vitest";
import { matchRoute } from "../src/route";

describe("route matching", () => {
  test("matches a valid session id", () => {
    expect(matchRoute("/s/abcdefgh")).toEqual({
      name: "terminal",
      id: "abcdefgh",
    });
    expect(matchRoute("/s/234567aa")).toEqual({
      name: "terminal",
      id: "234567aa",
    });
  });

  test("matches a valid code window id", () => {
    expect(matchRoute("/code/abcdefgh")).toEqual({
      name: "code",
      id: "abcdefgh",
    });
  });

  test("matches the settings pages", () => {
    expect(matchRoute("/settings")).toEqual({ name: "settings" });
    expect(matchRoute("/settings/tailscale")).toEqual({
      name: "settings-tailscale",
    });
    expect(matchRoute("/settings/tunnel")).toEqual({ name: "settings-tunnel" });
  });

  test("falls back to the session list", () => {
    expect(matchRoute("/")).toEqual({ name: "sessions" });
    expect(matchRoute("/unknown")).toEqual({ name: "sessions" });
  });

  test("keeps the current trailing-slash behavior", () => {
    expect(matchRoute("/settings/")).toEqual({ name: "sessions" });
    expect(matchRoute("/settings/tailscale/")).toEqual({ name: "sessions" });
    expect(matchRoute("/settings/tunnel/")).toEqual({ name: "sessions" });
    expect(matchRoute("/s/abcdefgh/")).toEqual({ name: "sessions" });
    expect(matchRoute("/code/abcdefgh/")).toEqual({ name: "sessions" });
  });

  test("rejects ids outside the base32 alphabet", () => {
    expect(matchRoute("/s/ABCDEFGH")).toEqual({ name: "sessions" });
    expect(matchRoute("/s/12345678")).toEqual({ name: "sessions" });
    expect(matchRoute("/s/abcdefg")).toEqual({ name: "sessions" });
    expect(matchRoute("/code/ABCDEFGH")).toEqual({ name: "sessions" });
  });
});
