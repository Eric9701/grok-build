import * as vscode from "vscode";

const ATLAS_SECTION = "atlas";
const LEGACY_SECTION = "grok";

/**
 * Read an extension setting from `atlas.*`, falling back to legacy `grok.*`
 * when the Atlas key was never set at any configuration target.
 */
export function getAtlasSetting<T>(key: string, defaultValue: T): T {
  const atlas = vscode.workspace.getConfiguration(ATLAS_SECTION);
  const inspected = atlas.inspect<T>(key);
  if (
    inspected?.globalValue !== undefined ||
    inspected?.workspaceValue !== undefined ||
    inspected?.workspaceFolderValue !== undefined
  ) {
    return atlas.get<T>(key, defaultValue) as T;
  }
  return vscode.workspace.getConfiguration(LEGACY_SECTION).get<T>(key, defaultValue) as T;
}

/** True if a configuration change event touches `atlas.<key>` or legacy `grok.<key>`. */
export function affectsAtlasSetting(e: vscode.ConfigurationChangeEvent, key: string): boolean {
  return e.affectsConfiguration(`${ATLAS_SECTION}.${key}`) || e.affectsConfiguration(`${LEGACY_SECTION}.${key}`);
}

/** VS Code configuration object for writing Atlas settings. */
export function atlasConfiguration(): vscode.WorkspaceConfiguration {
  return vscode.workspace.getConfiguration(ATLAS_SECTION);
}
