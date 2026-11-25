// Mirrors domain.NormalizeTagName so chips show what the server will store:
// trimmed, lower case, runs of whitespace/underscores become "-".
export function normalizeTag(raw: string): string {
  return raw.trim().toLowerCase().replace(/[\s_]+/g, '-');
}

export function isValidTag(name: string): boolean {
  return /^[a-z0-9][a-z0-9-]{0,29}$/.test(name);
}
