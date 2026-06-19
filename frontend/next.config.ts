import type { NextConfig } from "next";

const backendTarget =
  process.env.KYLIN_GUARD_AGENT_API_URL ||
  process.env.KYLINGUARD_AGENT_API_URL ||
  "http://127.0.0.1:8080";

const normalizedBackendTarget = backendTarget.replace(/\/$/, "");

const nextConfig: NextConfig = {
  async rewrites() {
    return [
      {
        source: "/api/agent/:path*",
        destination: `${normalizedBackendTarget}/api/agent/:path*`,
      },
      {
        source: "/api/os/:path*",
        destination: `${normalizedBackendTarget}/api/os/:path*`,
      },
      {
        source: "/health",
        destination: `${normalizedBackendTarget}/health`,
      },
    ];
  },
};

export default nextConfig;
