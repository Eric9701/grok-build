// DOM test for #223 / #225 — VS Code's Ctrl+C and Copy refocus the webview
// before they copy. The window-focus hook used to hand focus to the composer
// whenever it found <body>, which collapsed a highlight in the transcript, so
// the copy read the composer (or nothing) instead.
import { describe, it, expect } from "vitest";
import { bootWebview } from "./webview-harness";

describe("window focus with a transcript highlight (#223, #225)", () => {
  function setup() {
    const { window, doc } = bootWebview();
    const input = doc.getElementById("input") as HTMLTextAreaElement;
    const p = doc.createElement("p");
    p.textContent = "copy me from the transcript";
    doc.getElementById("messages")!.appendChild(p);
    input.value = "asdf";
    input.blur();
    expect(doc.activeElement).toBe(doc.body);
    return { window, doc, input, p };
  }

  it("leaves focus and the highlight where they are", () => {
    const { window, doc, p } = setup();
    const range = doc.createRange();
    range.selectNodeContents(p);
    window.getSelection()!.removeAllRanges();
    window.getSelection()!.addRange(range);

    window.dispatchEvent(new window.FocusEvent("focus"));

    expect(doc.activeElement).toBe(doc.body);
    expect(String(window.getSelection())).toBe("copy me from the transcript");
  });

  it("still claims the composer when nothing is highlighted", () => {
    const { window, doc, input } = setup();
    window.getSelection()!.removeAllRanges();

    window.dispatchEvent(new window.FocusEvent("focus"));

    expect(doc.activeElement).toBe(input);
  });

  // Ctrl+C / the workbench Copy command deliver a copy event. A focused
  // composer used to swallow it (its own caret is what the host copies), and
  // the refocus that precedes Copy can collapse the highlight before the
  // event — the selection is already gone, so the focus guard never sees it.
  function selectTranscript(window: Window, doc: Document, p: HTMLElement) {
    const range = doc.createRange();
    range.selectNodeContents(p);
    window.getSelection()!.removeAllRanges();
    window.getSelection()!.addRange(range);
    doc.dispatchEvent(new window.Event("selectionchange"));
  }

  function dispatchCopy(window: Window, doc: Document) {
    const store = new Map<string, string>();
    const clipboardData = {
      setData: (type: string, value: string) => { store.set(type, value); },
      getData: (type: string) => store.get(type) || "",
    };
    const ev = new window.Event("copy", { bubbles: true, cancelable: true });
    Object.defineProperty(ev, "clipboardData", { value: clipboardData });
    doc.dispatchEvent(ev);
    return { ev, clipboardData };
  }

  it("puts the transcript highlight on the clipboard", () => {
    const { window, doc, p } = setup();
    selectTranscript(window, doc, p);

    const { ev, clipboardData } = dispatchCopy(window, doc);

    expect(ev.defaultPrevented).toBe(true);
    expect(clipboardData.getData("text/plain")).toBe("copy me from the transcript");
  });

  it("copies the highlight after the refocus collapsed it", () => {
    const { window, doc, input, p } = setup();
    selectTranscript(window, doc, p);
    window.getSelection()!.removeAllRanges();
    input.focus();
    input.dispatchEvent(new window.KeyboardEvent("keydown", { key: "c", ctrlKey: true, bubbles: true }));

    const { clipboardData } = dispatchCopy(window, doc);

    expect(clipboardData.getData("text/plain")).toBe("copy me from the transcript");
    expect(input.selectionStart).toBe(input.selectionEnd);
  });

  it("does not override a range the user selected in the composer", () => {
    const { window, doc, input, p } = setup();
    selectTranscript(window, doc, p);
    input.dispatchEvent(new window.Event("pointerdown", { bubbles: true }));
    window.getSelection()!.removeAllRanges();
    input.focus();
    input.setSelectionRange(0, input.value.length);

    const { ev, clipboardData } = dispatchCopy(window, doc);

    expect(ev.defaultPrevented).toBe(false);
    expect(clipboardData.getData("text/plain")).toBe("");
  });

  it("does not reuse a highlight after the user types in the composer", () => {
    const { window, doc, input, p } = setup();
    selectTranscript(window, doc, p);
    window.getSelection()!.removeAllRanges();
    input.focus();
    input.dispatchEvent(new window.KeyboardEvent("keydown", { key: "x", bubbles: true }));

    const { ev, clipboardData } = dispatchCopy(window, doc);

    expect(ev.defaultPrevented).toBe(false);
    expect(clipboardData.getData("text/plain")).toBe("");
  });
});
