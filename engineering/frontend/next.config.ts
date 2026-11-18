import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: 'standalone',

  outputFileTracingRoot: import.meta.dirname,
  turbopack: {
    root: import.meta.dirname,
  },
  experimental: {
    serverActions: {
      allowedOrigins: ["https://8af8753d-8243-4d6e-9952-f133a694bd15.e1-us-east-azure.choreoapps.dev"]
    }
  }
};

export default nextConfig;
