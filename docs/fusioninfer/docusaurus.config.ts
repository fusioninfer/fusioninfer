import { themes as prismThemes } from 'prism-react-renderer';
import type { Config } from '@docusaurus/types';
import type * as Preset from '@docusaurus/preset-classic';

const config: Config = {
  title: 'FusionInfer',
  tagline: 'Unified Kubernetes-native LLM inference platform with intelligent routing and gang scheduling',
  favicon: 'img/favicon.ico',

  future: {
    v4: true,
  },

  // For GitHub Pages deployment
  // If using custom domain: url: 'https://fusioninfer.io', baseUrl: '/'
  // If using github.io: url: 'https://<org>.github.io', baseUrl: '/<repo>/'
  url: 'https://fusioninfer.github.io',
  baseUrl: '/fusioninfer/',

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
        blog: {
          showReadingTime: true,
          feedOptions: {
            type: ['rss', 'atom'],
            xslt: true,
          },
          editUrl: 'https://github.com/fusioninfer/fusioninfer/tree/main/docs/fusioninfer/',
          onInlineTags: 'warn',
          onInlineAuthors: 'warn',
          onUntruncatedBlogPosts: 'warn',
        },
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
        { to: '/blog', label: 'Blog', position: 'left' },
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
              label: 'Blog',
              to: '/blog',
            },
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

