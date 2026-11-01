// The article key the server returns for a generation start. The extension
// never interprets its form; it only bounds its length, the same everywhere a
// key is received.

declare const articleKeyBrand: unique symbol;

/** A non-empty string of at most maxKeyLength code points. Only parseArticleKey creates it. */
export type ArticleKey = string & { readonly [articleKeyBrand]: true };

/** Above the server's 64-character keys, so a key the server returned is never refused later. */
export const maxKeyLength = 256;

/** Returns the value unchanged when it is a non-empty string within the limit; otherwise undefined. */
export function parseArticleKey(value: unknown): ArticleKey | undefined {
  if (typeof value !== "string" || value === "") {
    return undefined;
  }
  if ([...value].length > maxKeyLength) {
    return undefined;
  }
  return value as ArticleKey;
}
