import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: 'standalone',

  outputFileTracingRoot: import.meta.dirname,
  turbopack: {
    root: import.meta.dirname,
  },
  experimental: {
    serverActions: {
      // Host only (no https://). Choreo's gateway sets x-forwarded-host to the
      // internal service name, so the public host has to be allowed explicitly.
      allowedOrigins: ["8af8753d-8243-4d6e-9952-f133a694bd15.e1-us-east-azure.choreoapps.dev"]
    }
  }
};

export default nextConfig;
