import { createHash, createPublicKey } from "node:crypto";

/**
 * The Chrome extension ID for a manifest "key": the first 16 bytes of the
 * SHA-256 of the DER SubjectPublicKeyInfo, in hex, with each hex digit 0-f
 * rewritten to a-p. Throws when the value is not a parseable public key,
 * which is how a pasted private key is rejected.
 */
export function extensionIdFromManifestKey(key: string): string {
  const publicKey = createPublicKey({
    key: Buffer.from(key, "base64"),
    format: "der",
    type: "spki",
  });
  const der = publicKey.export({ format: "der", type: "spki" });
  const hex = createHash("sha256").update(der).digest("hex").slice(0, 32);
  return Array.from(hex, (digit) =>
    String.fromCharCode("a".charCodeAt(0) + Number.parseInt(digit, 16)),
  ).join("");
}
