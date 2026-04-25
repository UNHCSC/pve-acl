import { mkdir, copyFile } from "node:fs/promises";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import sharp from "sharp";

const root = dirname(dirname(fileURLToPath(import.meta.url)));
const source = join(root, "src", "assets", "organesson-logo.svg");
const staticDir = join(root, "static");

await mkdir(staticDir, { recursive: true });
await copyFile(source, join(staticDir, "logo.svg"));
await copyFile(source, join(staticDir, "favicon.svg"));

await Promise.all([
    sharp(source).resize(32, 32).png().toFile(join(staticDir, "favicon-32.png")),
    sharp(source).resize(180, 180).png().toFile(join(staticDir, "apple-touch-icon.png"))
]);
