// View placement. The view is default-homed in the SECONDARY side bar
// (`viewsContainers.secondarySidebar`, VS Code >= 1.106 — hence the engines
// floor), with one extension-owned container contributed per dock location so
// the gear-menu "Move view" items can move the view DIRECTLY via the internal
// `vscode.moveViews` command (the one GitLens uses for its layout switch) — no
// quickpick. This exists because Cursor's primary-side-bar context menu hides
// the built-in "Move To" entry, and a one-click mover is useful everywhere.

export const GROK_VIEW_ID = "atlas.chat";

/** Contributed containers, one per dock location (package.json prefixes each id
 *  with `workbench.view.extension.`). `atlasSidebar` homes the view; the other
 *  two are empty by default (an empty container renders nothing) and exist only
 *  as `vscode.moveViews` targets. */
export const SECONDARY_CONTAINER_ID = "workbench.view.extension.atlasSidebar";
export const PRIMARY_CONTAINER_ID = "workbench.view.extension.atlasPrimary";
export const PANEL_CONTAINER_ID = "workbench.view.extension.atlasPanel";

/** Resolve a gear-menu destination to the container `vscode.moveViews` should
 *  target, or null for an unknown location (callers fall back to the built-in
 *  destination picker preselected on the Atlas view). */
export function moveViewContainerFor(location: unknown): string | null {
  if (location === "panel") return PANEL_CONTAINER_ID;
  if (location === "sidebar") return PRIMARY_CONTAINER_ID;
  if (location === "auxiliarybar") return SECONDARY_CONTAINER_ID;
  return null;
}
