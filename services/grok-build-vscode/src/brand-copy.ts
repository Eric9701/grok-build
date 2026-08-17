/**
 * User-visible product copy. Protocol ids (`grok`, `grok-build`, `GROK_*`)
 * stay on the wire; SuperGrok is an xAI plan name and is left alone.
 */

export function brandUserFacingText(text: string): string {
  return String(text)
    .replace(/\bGrok Build Desktop\b/gi, "Atlas Desktop")
    .replace(/\bGrok Build CLI\b/gi, "Atlas CLI")
    .replace(/\bGrok Build\b/gi, "Atlas")
    .replace(/\bGrok CLI\b/gi, "Atlas CLI");
}

/** Human label for a model row. Raw ids like `grok-4.5` stay as-is when no name. */
export function brandModelDisplayName(name: string | undefined, modelId?: string): string {
  const id = String(modelId || "").trim();
  const rawName = String(name || "").trim();
  if (!rawName) return /^grok-build$/i.test(id) ? "Atlas" : id;
  let branded = brandUserFacingText(rawName);
  if (/^Grok\b/i.test(branded) && !/^SuperGrok\b/i.test(branded)) {
    branded = branded.replace(/^Grok\b/i, "Atlas");
  }
  return branded;
}
