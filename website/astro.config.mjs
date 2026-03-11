import { defineConfig } from 'astro/config';
import tailwind from '@astrojs/tailwind';
import mdx from '@astrojs/mdx';
import remarkGithubAlerts from 'remark-github-alerts';

export default defineConfig({
  site: 'https://trala.fyi',
  srcDir: './src',
  outDir: '../docs/dist',
  publicDir: './public',
  integrations: [
    tailwind(),
    mdx({syntaxHighlight: 'shiki'})
  ],
  markdown: {
    shikiConfig: {
      themes: {
        light: 'github-light',
        dark: 'dracula'
      }
    },
    remarkPlugins: [remarkGithubAlerts]
  }
});
