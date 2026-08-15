import type { NextConfig } from "next";
import { fileURLToPath } from "node:url";
import { ADMIN_BASE_PATH } from "./admin-path";

const nextConfig: NextConfig = {
  allowedDevOrigins: ["127.0.0.1", "localhost"],
  basePath: ADMIN_BASE_PATH,
  output: "standalone",
  outputFileTracingRoot: fileURLToPath(new URL("../../", import.meta.url)),
  async redirects() {
    return [{ source: "/", destination: ADMIN_BASE_PATH, permanent: false, basePath: false }];
  },
};

export default nextConfig;
