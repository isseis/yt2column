// Copies static/ into dist/ after tsc has written the compiled sources. A
// missing static/ copies nothing. A static file whose path collides with a
// compiled file fails the build instead of overwriting it.
import { cpSync, existsSync } from "node:fs";

if (existsSync("static")) {
  cpSync("static", "dist", {
    recursive: true,
    force: false,
    errorOnExist: true,
  });
}
