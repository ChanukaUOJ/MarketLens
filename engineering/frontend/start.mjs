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
import {
  accessSync,
  constants,
  cpSync,
  existsSync,
  readFileSync,
  readdirSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const appDir = dirname(fileURLToPath(import.meta.url));
// Used when the app directory is read-only: the app is copied here and the
// values are filled in on the copy. /tmp is writable on Choreo.
const writableAppDir = "/tmp/app";

const configFile = process.env.CONFIG_FILE || "/app/configs/config.json";

// Placeholders compiled in by Dockerfile.choreo. Each must be given a value.
const BUILD_PLACEHOLDERS = [
  "NEXT_PUBLIC_THUNDERID_BASE_URL",
  "NEXT_PUBLIC_THUNDERID_APPLICATION_ID",
  "NEXT_PUBLIC_THUNDERID_CLIENT_ID",
  "NEXT_PUBLIC_THUNDERID_AFTER_SIGN_IN_URL",
  "NEXT_PUBLIC_THUNDERID_AFTER_SIGN_OUT_URL",
];

// List placeholders compiled in by Dockerfile.choreo. The build stores each as
// a single JSON string in a list (["__KEY__"]); the comma-separated value from
// the config replaces it with one entry per item.
//   SERVER_ACTIONS_ALLOWED_ORIGINS: the public host(s) of this frontend, which
//   next.config.ts passes to serverActions.allowedOrigins (sign-in needs it).
const LIST_PLACEHOLDERS = ["SERVER_ACTIONS_ALLOWED_ORIGINS"];

// Same normalisation as next.config.ts: keep only the host of each entry.
function hostList(value) {
  return value
    .split(",")
    .map((entry) => entry.trim().replace(/^https?:\/\//, "").replace(/\/.*$/, ""))
    .filter(Boolean);
}

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

function isWritable(dir) {
  try {
    accessSync(dir, constants.W_OK);
    return true;
  } catch {
    return false;
  }
}

// The build output plus server.js, which holds its own copy of next.config.
function* buildFiles(dir) {
  yield* walk(join(dir, ".next"));
  yield join(dir, "server.js");
}

// Returns the directory to start the server from.
function fillPlaceholders() {
  const placeholders = [
    ...LIST_PLACEHOLDERS.map((key) => `"__${key}__"`),
    ...BUILD_PLACEHOLDERS.map((key) => `__${key}__`),
  ];
  const hasPlaceholder = (file) => {
    const text = readFileSync(file, "utf8");
    return placeholders.some((placeholder) => text.includes(placeholder));
  };

  // Nothing to do if the build has already been filled in (container restart).
  if (![...buildFiles(appDir)].some(hasPlaceholder)) return appDir;

  const missing = [...BUILD_PLACEHOLDERS, ...LIST_PLACEHOLDERS].filter(
    (key) => !process.env[key] || (LIST_PLACEHOLDERS.includes(key) && hostList(process.env[key]).length === 0),
  );
  if (missing.length > 0) {
    fail(`missing in ${configFile}: ${missing.join(", ")}`);
  }

  // List placeholders first: their quoted form contains the plain one.
  const replacements = [
    ...LIST_PLACEHOLDERS.map((key) => [
      `"__${key}__"`,
      hostList(process.env[key]).map((host) => JSON.stringify(host)).join(","),
    ]),
    ...BUILD_PLACEHOLDERS.map((key) => [`__${key}__`, process.env[key]]),
  ];

  // Fill in place if possible; if the filesystem is read-only, work on a
  // copy of the app in /tmp instead (without the mounted configs folder).
  let targetDir = appDir;
  if (!isWritable(join(appDir, ".next"))) {
    console.log(`${appDir} is read-only; copying the app to ${writableAppDir}`);
    rmSync(writableAppDir, { recursive: true, force: true });
    cpSync(appDir, writableAppDir, {
      recursive: true,
      filter: (src) => !src.startsWith(join(appDir, "configs")),
    });
    targetDir = writableAppDir;
  }

  const pending = [...buildFiles(targetDir)].filter(hasPlaceholder);
  for (const file of pending) {
    let text = readFileSync(file, "utf8");
    for (const [placeholder, value] of replacements) {
      text = text.split(placeholder).join(value);
    }
    try {
      writeFileSync(file, text);
    } catch (err) {
      fail(`could not write ${file} (${err.code})`);
    }
  }
  console.log(`Filled in config values in ${pending.length} build files`);
  return targetDir;
}

loadConfig();
const serverDir = fillPlaceholders();
// server.js changes into its own directory, so it serves the filled-in copy.
await import(join(serverDir, "server.js"));
