import * as vscode from "vscode";
import { GrokSidebar } from "./sidebar";

/** What `activate` hands back through `extension.exports`. Empty in every
 *  released build — the test seam below is populated only under
 *  `ExtensionMode.Test`. */
export interface GrokExtensionApi {
  __test?: ReturnType<GrokSidebar["installTestHooks"]>;
}

export function activate(context: vscode.ExtensionContext): GrokExtensionApi {
  const output = vscode.window.createOutputChannel("Atlas");
  const sidebar = new GrokSidebar(context, output);

  context.subscriptions.push(
    vscode.window.registerWebviewViewProvider(GrokSidebar.viewId, sidebar, {
      webviewOptions: { retainContextWhenHidden: true },
    }),
    output,
    { dispose: () => sidebar.dispose() },
    vscode.commands.registerCommand("atlas.open", () =>
      vscode.commands.executeCommand("workbench.view.extension.atlasSidebar"),
    ),
    vscode.commands.registerCommand("atlas.newSession", () => sidebar.newSession()),
    vscode.commands.registerCommand("atlas.newWorktreeSession", () => sidebar.newWorktreeSession()),
    vscode.commands.registerCommand("atlas.applyWorktree", () => sidebar.applyFocusedWorktree()),
    vscode.commands.registerCommand("atlas.removeWorktree", () => sidebar.removeFocusedWorktree()),
    vscode.commands.registerCommand("atlas.rewind", () => sidebar.rewindFocusedSession()),
    vscode.commands.registerCommand("atlas.compact", () => {
      // emulated by sending the slash command as a prompt; CLI handles it
      vscode.window.showInformationMessage(
        "Type /compact in the composer to compress the conversation.",
      );
    }),
    vscode.commands.registerCommand("atlas.pickModel", () => sidebar.pickModel()),
    vscode.commands.registerCommand("atlas.toggleMode", () => sidebar.openModePopover()),
    vscode.commands.registerCommand("atlas.sendSelection", () =>
      sidebar.insertActiveMention({ selection: true }),
    ),
    vscode.commands.registerCommand(
      "atlas.sendFile",
      (uri?: vscode.Uri) => sidebar.insertActiveMention({ uri, pickIfMissing: true }),
    ),
    vscode.commands.registerCommand("atlas.insertAtMention", () =>
      sidebar.insertActiveMention(),
    ),
    vscode.commands.registerCommand("atlas.showLogs", () => output.show()),
    vscode.commands.registerCommand("atlas.expandAllToolDetails", () => sidebar.setAllToolDetails(true)),
    vscode.commands.registerCommand("atlas.collapseAllToolDetails", () => sidebar.setAllToolDetails(false)),
    vscode.commands.registerCommand("atlas.logout", () => sidebar.logout()),
    vscode.commands.registerCommand("atlas.linkRemote", () => sidebar.linkRemoteDevice()),
    vscode.commands.registerCommand("atlas.unlinkRemote", () => sidebar.unlinkRemoteDevice()),
    vscode.commands.registerCommand("atlas.composerForward", () => sidebar.moveComposerCaret("forward")),
    vscode.commands.registerCommand("atlas.composerPreviousLine", () => sidebar.moveComposerCaret("previousLine")),
    // Internal debug helper for manually exercising the plan-review card UI
    // (Approve / Reject / Cancel flows) without a live CLI session.
    vscode.commands.registerCommand("atlas._debugDummyPlan", () => sidebar.debugShowDummyPlan()),
  );

  // VS Code sets ExtensionMode.Test ONLY when the extension host was launched by
  // a test runner, so an installed build can never reach this branch and the
  // seam is genuinely absent there rather than merely undocumented.
  return context.extensionMode === vscode.ExtensionMode.Test
    ? { __test: sidebar.installTestHooks() }
    : {};
}

export function deactivate(): void {
  // disposables handle cleanup
}
