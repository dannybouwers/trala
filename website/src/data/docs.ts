// Shared documentation page order - used by Sidebar and PageNavigation
export const docs = [
  { path: '/docs/README', title: 'Quick Start' },
  { path: '/docs/setup', title: 'Setup' },
  { path: '/docs/configuration', title: 'Configuration' },
  { path: '/docs/services', title: 'Services' },
  { path: '/docs/grouping', title: 'Grouping' },
  { path: '/docs/icons', title: 'Icons' },
  { path: '/docs/search', title: 'Search' },
  { path: '/docs/security', title: 'Security' },
  { path: '/docs/development', title: 'Development' },
] as const;

export type Doc = typeof docs[number];
