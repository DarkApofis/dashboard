import type { NextConfig } from 'next';

const config: NextConfig = {
  reactStrictMode: true,
  // Emits a self-contained server bundle, so the runtime image needs neither
  // node_modules nor the sources.
  output: 'standalone',
};

export default config;
