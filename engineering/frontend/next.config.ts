import type { NextConfig } from "next";

// Extra hosts allowed to call Server Actions (used by the ThunderID sign-in and
// sign-up). Behind Choreo's gateway, x-forwarded-host is the internal service
// name, so Next.js only accepts the action if the public host is listed here.
//
// Comma-separated hosts, e.g. "abc.e1-us-east-azure.choreoapps.dev". A full
// URL is accepted too; only its host is kept. Unset (local dev) = same-origin
// only. On Choreo, Dockerfile.choreo builds with a placeholder that start.mjs
// fills in from /app/configs/config.json.
const allowedOrigins = (process.env.SERVER_ACTIONS_ALLOWED_ORIGINS ?? "")
  .split(",")
  .map((entry) => entry.trim().replace(/^https?:\/\//, "").replace(/\/.*$/, ""))
  .filter(Boolean);

const nextConfig: NextConfig = {
  output: 'standalone',

  outputFileTracingRoot: import.meta.dirname,
  turbopack: {
    root: import.meta.dirname,
  },
  experimental: {
    serverActions: {
      allowedOrigins,
    },
  },
};

export default nextConfig;
