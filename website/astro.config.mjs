import { defineConfig } from 'astro/config';
import { unified } from '@astrojs/markdown-remark';
import mdx from '@astrojs/mdx';
import remarkGithubAlerts from 'remark-github-alerts';
import tailwindcss from '@tailwindcss/vite';

export default defineConfig({
  site: 'https://trala.fyi',
  srcDir: './src',
  outDir: './dist',
  publicDir: './public',

  integrations: [
    mdx({syntaxHighlight: 'shiki'})
  ],

  markdown: {
    processor: unified({
      remarkPlugins: [remarkGithubAlerts]
    }),
    shikiConfig: {
      themes: {
        light: 'github-light',
        dark: 'dracula'
      }
    }
  },

  vite: {
    plugins: [tailwindcss()]
  }
});