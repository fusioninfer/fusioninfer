import { themes as prismThemes } from 'prism-react-renderer';
import type { Config } from '@docusaurus/types';
import type * as Preset from '@docusaurus/preset-classic';

const config: Config = {
  title: 'FusionInfer',
  tagline: 'A Kubernetes-native platform for orchestrating distributed LLM inference at scale',
  favicon: 'img/favicon.ico',

  future: {
    v4: true,
  },

  // For GitHub Pages deployment with custom domain
  url: 'https://fusioninfer.github.io',
  baseUrl: '/',

  organizationName: 'fusioninfer',
  projectName: 'fusioninfer',
  deploymentBranch: 'gh-pages',

  onBrokenLinks: 'throw',
  onBrokenAnchors: 'warn',
  trailingSlash: false,

  i18n: {
    defaultLocale: 'en',
    locales: ['en'],
  },

  markdown: {
    format: 'mdx',
    mermaid: true,
    hooks: {
      onBrokenMarkdownLinks: 'warn',
    },
  },

  presets: [
    [
      'classic',
      {
        docs: {
          sidebarPath: './sidebars.ts',
          editUrl: 'https://github.com/fusioninfer/fusioninfer/tree/main/docs/fusioninfer/',
        },
        blog: false,
        theme: {
          customCss: './src/css/custom.css',
        },
      } satisfies Preset.Options,
    ],
  ],

  themes: ['@docusaurus/theme-mermaid'],

  themeConfig: {
    image: 'img/fusioninfer-social-card.png',
    navbar: {
      title: 'FusionInfer',
      logo: {
        alt: 'FusionInfer Logo',
        src: 'img/logo.svg',
      },
      items: [
        {
          type: 'docSidebar',
          sidebarId: 'tutorialSidebar',
          position: 'left',
          label: 'Docs',
        },
        {
          href: 'https://github.com/fusioninfer/fusioninfer',
          label: 'GitHub',
          position: 'right',
        },
      ],
    },
    footer: {
      style: 'dark',
      links: [
        {
          title: 'Docs',
          items: [
            {
              label: 'Introduction',
              to: '/docs/intro',
            },
            {
              label: 'Design',
              to: '/docs/design/core-design',
            },
          ],
        },
        {
          title: 'Community',
          items: [
            {
              label: 'GitHub Issues',
              href: 'https://github.com/fusioninfer/fusioninfer/issues',
            },
            {
              label: 'Discussions',
              href: 'https://github.com/fusioninfer/fusioninfer/discussions',
            },
          ],
        },
        {
          title: 'More',
          items: [
            {
              label: 'GitHub',
              href: 'https://github.com/fusioninfer/fusioninfer',
            },
          ],
        },
      ],
      copyright: `Copyright © ${new Date().getFullYear()} FusionInfer. Built with Docusaurus.`,
    },
    prism: {
      theme: prismThemes.github,
      darkTheme: prismThemes.dracula,
      additionalLanguages: ['bash', 'yaml', 'go'],
    },
  } satisfies Preset.ThemeConfig,
};

export default config;

