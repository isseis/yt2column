// Runs before `npm ci`, so it must use nothing but Node.js built-ins.
import { existsSync, readFileSync } from "node:fs";

export const registryPrefix = "https://registry.npmjs.org/";

/**
 * Returns one message per lockfile entry that npm would fetch from anywhere
 * other than the public registry, or from the registry under another
 * package's name. An entry without a "resolved" URL is also reported: npm
 * would pick its source from configuration, not from the lockfile. An empty
 * result means every package comes from the registry under its own name.
 */
export function checkLockfile(lockfile: unknown): string[] {
  if (typeof lockfile !== "object" || lockfile === null) {
    return ["lockfile is not a JSON object"];
  }
  if (!("lockfileVersion" in lockfile) || lockfile.lockfileVersion !== 3) {
    return ["lockfileVersion is not 3"];
  }
  if (!("packages" in lockfile)) {
    return ['lockfile has no "packages" object'];
  }
  const packages = lockfile.packages;
  if (typeof packages !== "object" || packages === null) {
    return ['lockfile has no "packages" object'];
  }
  const violations: string[] = [];
  for (const [path, entry] of Object.entries(packages)) {
    if (path === "") {
      continue; // The root project itself is not fetched.
    }
    const resolved: unknown =
      typeof entry === "object" && entry !== null && "resolved" in entry
        ? entry.resolved
        : undefined;
    if (typeof resolved !== "string") {
      violations.push(`${path}: no "resolved" URL`);
    } else if (!resolved.startsWith(registryPrefix)) {
      violations.push(
        `${path}: resolved outside ${registryPrefix}: ${resolved}`,
      );
    } else {
      // "node_modules/a/node_modules/@s/b" installs the package "@s/b".
      const marker = "node_modules/";
      const at = path.lastIndexOf(marker);
      const name = at < 0 ? undefined : path.slice(at + marker.length);
      if (
        name === undefined ||
        !resolved.startsWith(`${registryPrefix}${name}/-/`)
      ) {
        violations.push(`${path}: resolved to another package: ${resolved}`);
      }
    }
  }
  return violations;
}

if (import.meta.main) {
  // npm ci reads npm-shrinkwrap.json in preference to package-lock.json, so
  // its presence would bypass this check.
  if (existsSync("npm-shrinkwrap.json")) {
    console.error(
      "check-lockfile: npm-shrinkwrap.json is not allowed; use package-lock.json",
    );
    process.exit(1);
  }
  const violations = checkLockfile(
    JSON.parse(readFileSync("package-lock.json", "utf8")),
  );
  if (violations.length > 0) {
    console.error(
      "check-lockfile: package-lock.json fetches packages from outside the registry:",
    );
    for (const violation of violations) {
      console.error(`  ${violation}`);
    }
    process.exit(1);
  }
}
