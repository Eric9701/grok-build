import { describe, expect, it } from "vitest";
import { brandModelDisplayName, brandUserFacingText } from "../src/brand-copy";

describe("brandUserFacingText", () => {
  it("rewrites product names and leaves SuperGrok alone", () => {
    expect(brandUserFacingText("Update Grok Build CLI")).toBe("Update Atlas CLI");
    expect(brandUserFacingText("You've reached your free Grok Build usage limit")).toBe(
      "You've reached your free Atlas usage limit",
    );
    expect(brandUserFacingText("Grok is still working")).toBe("Atlas is still working");
    expect(brandUserFacingText("Get SuperGrok for higher limits")).toBe("Get SuperGrok for higher limits");
    expect(brandUserFacingText("The model 'grok-build' requires a Grok subscription.")).toBe(
      "The model 'grok-build' requires an Atlas subscription.",
    );
    expect(brandUserFacingText("Run `grok login` or add api_key to ~/.grok/config.toml.")).toBe(
      "Run `atlas login` or add api_key to ~/.atlas/config.toml.",
    );
    expect(brandUserFacingText("Grok-Build-Desktop-4.6.0-win-x64.exe")).toBe(
      "Grok-Build-Desktop-4.6.0-win-x64.exe",
    );
  });
});

describe("brandModelDisplayName", () => {
  it("maps the default grok-build product name to Atlas", () => {
    expect(brandModelDisplayName("Grok Build", "grok-build")).toBe("Atlas");
    expect(brandModelDisplayName(undefined, "grok-build")).toBe("Atlas");
  });

  it("rewrites Grok-prefixed display names but keeps raw ids", () => {
    expect(brandModelDisplayName("Grok 4.5", "grok-4.5")).toBe("Atlas 4.5");
    expect(brandModelDisplayName(undefined, "grok-4.5")).toBe("grok-4.5");
    expect(brandModelDisplayName("Composer 2.5 Fast", "grok-composer-2.5-fast")).toBe("Composer 2.5 Fast");
  });
});
