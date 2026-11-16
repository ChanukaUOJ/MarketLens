// Startup script for running the frontend on Choreo as a Web Application.
//
// Choreo web applications can't set environment variables and only offer a
// file mount at /app/configs/config.json (a flat JSON object of KEY: value
// pairs). This script:
//   1. loads that file into process.env (server-side settings such as
//      THUNDERID_SECRET, THUNDERID_CLIENT_SECRET, BACKEND_API_URL);
//   2. replaces the __NEXT_PUBLIC_..._ placeholders that Dockerfile.choreo
//      compiled into the build with the real values from the same file, so
//      those values are never in the repo or the image;
//   3. starts the Next.js standalone server.
//
// The Next.js server only serves public/ and .next/static, so the mounted
// file itself is not reachable from a browser. (The NEXT_PUBLIC_* values do
// reach the browser, as they do in any Next.js app.)
//
// If the file isn't there (e.g. running locally), the server just uses the
// environment it was started with.
import { existsSync, readFileSync, readdirSync, writeFileSync } from "node:fs";
import { join } from "node:path";

const configFile = process.env.CONFIG_FILE || "/app/configs/config.json";

// Placeholders compiled in by Dockerfile.choreo. Each must be given a value.
const BUILD_PLACEHOLDERS = [
  "NEXT_PUBLIC_THUNDERID_BASE_URL",
  "NEXT_PUBLIC_THUNDERID_APPLICATION_ID",
  "NEXT_PUBLIC_THUNDERID_CLIENT_ID",
  "NEXT_PUBLIC_THUNDERID_AFTER_SIGN_IN_URL",
];

function fail(message) {
  console.error(`Error: ${message}`);
  process.exit(1);
}

function loadConfig() {
  if (!existsSync(configFile)) return null;
  let config;
  try {
    config = JSON.parse(readFileSync(configFile, "utf8"));
  } catch (err) {
    fail(`${configFile} is not valid JSON: ${err.message}`);
  }
  if (config === null || typeof config !== "object" || Array.isArray(config)) {
    fail(`${configFile} must be a JSON object of KEY: value pairs`);
  }
  for (const [key, value] of Object.entries(config)) {
    process.env[key] = typeof value === "string" ? value : JSON.stringify(value);
  }
  console.log(`Loaded settings from ${configFile}`);
  return config;
}

// Files in the Next.js build output that can contain compiled-in values.
const TEXT_FILE = /\.(js|mjs|cjs|json|html|rsc|body|meta|txt)$/;

function* walk(dir) {
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const path = join(dir, entry.name);
    if (entry.isDirectory()) yield* walk(path);
    else if (TEXT_FILE.test(entry.name)) yield path;
  }
}

function fillPlaceholders() {
  const replacements = BUILD_PLACEHOLDERS.map((key) => [`__${key}__`, process.env[key]]);

  // Skip if the build has already been filled in (container restart).
  const files = [...walk(".next")];
  const pending = files.filter((file) => {
    const text = readFileSync(file, "utf8");
    return replacements.some(([placeholder]) => text.includes(placeholder));
  });
  if (pending.length === 0) return;

  const missing = BUILD_PLACEHOLDERS.filter((key) => !process.env[key]);
  if (missing.length > 0) {
    fail(`missing in ${configFile}: ${missing.join(", ")}`);
  }

  for (const file of pending) {
    let text = readFileSync(file, "utf8");
    for (const [placeholder, value] of replacements) {
      text = text.split(placeholder).join(value);
    }
    try {
      writeFileSync(file, text);
    } catch (err) {
      fail(`could not write ${file} (${err.code}). The app directory must be writable to fill in NEXT_PUBLIC_* values.`);
    }
  }
  console.log(`Filled in NEXT_PUBLIC_* values in ${pending.length} build files`);
}

loadConfig();
fillPlaceholders();
await import("./server.js");
