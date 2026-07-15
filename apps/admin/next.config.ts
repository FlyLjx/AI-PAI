import type { NextConfig } from "next";
import { fileURLToPath } from "node:url";
import { ADMIN_BASE_PATH } from "./admin-path";

const nextConfig: NextConfig = {
  basePath: ADMIN_BASE_PATH,
  output: "standalone",
  outputFileTracingRoot: fileURLToPath(new URL("../../", import.meta.url)),
};

export default nextConfig;
