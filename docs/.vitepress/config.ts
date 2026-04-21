import { defineConfig } from 'vitepress'

const repo = 'rpcduel'
const repoUrl = `https://github.com/xueqianLu/${repo}`

export default defineConfig({
  lang: 'en-US',
  title: 'rpcduel',
  description:
    'A high-performance CLI for comparing and benchmarking Ethereum JSON-RPC endpoints.',
  // Site is published to https://xueqianlu.github.io/rpcduel/
  base: `/${repo}/`,
  cleanUrls: true,
  lastUpdated: true,
  ignoreDeadLinks: 'localhostLinks',

  head: [
    ['link', { rel: 'icon', href: `/${repo}/favicon.svg`, type: 'image/svg+xml' }],
    ['meta', { name: 'theme-color', content: '#3c8772' }],
    ['meta', { property: 'og:type', content: 'website' }],
    ['meta', { property: 'og:title', content: 'rpcduel' }],
    [
      'meta',
      {
        property: 'og:description',
        content:
          'Compare, diff, benchmark and replay Ethereum JSON-RPC endpoints from a single binary.',
      },
    ],
  ],

  themeConfig: {
    logo: '/logo.svg',

    nav: [
      { text: 'Guide', link: '/guide/getting-started', activeMatch: '/guide/' },
      { text: 'Commands', link: '/commands/call', activeMatch: '/commands/' },
      {
        text: 'Data-Driven ⭐',
        link: '/data-driven/workflow',
        activeMatch: '/data-driven/',
      },
      { text: 'Advanced', link: '/advanced/config', activeMatch: '/advanced/' },
      { text: 'Reference', link: '/reference/output-formats', activeMatch: '/reference/' },
      {
        text: 'Releases',
        link: `${repoUrl}/releases`,
      },
    ],

    sidebar: {
      '/guide/': [
        {
          text: 'Guide',
          items: [
            { text: 'Why rpcduel?', link: '/guide/getting-started' },
            { text: 'Installation', link: '/guide/installation' },
            { text: 'Global flags', link: '/guide/global-flags' },
          ],
        },
        {
          text: 'Next steps',
          items: [
            { text: 'Basic commands', link: '/commands/call' },
            { text: 'Data-driven workflow', link: '/data-driven/workflow' },
          ],
        },
      ],
      '/commands/': [
        {
          text: 'Basic Commands',
          items: [
            { text: 'call', link: '/commands/call' },
            { text: 'diff', link: '/commands/diff' },
            { text: 'bench', link: '/commands/bench' },
            { text: 'duel', link: '/commands/duel' },
          ],
        },
        {
          text: 'Going further',
          items: [
            { text: 'Data-driven workflow ⭐', link: '/data-driven/workflow' },
            { text: 'Advanced features', link: '/advanced/config' },
          ],
        },
      ],
      '/data-driven/': [
        {
          text: 'Data-Driven Testing ⭐',
          items: [
            { text: 'Why & Workflow', link: '/data-driven/workflow' },
            { text: 'dataset', link: '/data-driven/dataset' },
            { text: 'replay', link: '/data-driven/replay' },
            { text: 'benchgen', link: '/data-driven/benchgen' },
          ],
        },
        {
          text: 'Reference',
          items: [
            { text: 'Dataset file format', link: '/reference/dataset-format' },
            { text: 'Output formats', link: '/reference/output-formats' },
          ],
        },
      ],
      '/advanced/': [
        {
          text: 'Advanced Features',
          items: [
            { text: 'Configuration file', link: '/advanced/config' },
            { text: 'SLO thresholds & CI gating', link: '/advanced/thresholds' },
            { text: 'Reports (HTML / MD / JUnit)', link: '/advanced/reports' },
            { text: 'Prometheus & Pushgateway', link: '/advanced/metrics' },
            { text: 'doctor — capability check', link: '/advanced/doctor' },
            { text: 'CI templates', link: '/advanced/ci' },
            { text: 'Shell completions & man pages', link: '/advanced/completions' },
          ],
        },
      ],
      '/reference/': [
        {
          text: 'Reference',
          items: [
            { text: 'Output formats', link: '/reference/output-formats' },
            { text: 'Dataset file format', link: '/reference/dataset-format' },
            { text: 'Architecture', link: '/reference/architecture' },
          ],
        },
      ],
    },

    socialLinks: [{ icon: 'github', link: repoUrl }],

    editLink: {
      pattern: `${repoUrl}/edit/main/docs/:path`,
      text: 'Edit this page on GitHub',
    },

    search: {
      provider: 'local',
    },

    footer: {
      message: 'Released under the MIT License.',
      copyright: `Copyright © ${new Date().getFullYear()} rpcduel contributors`,
    },
  },
})
