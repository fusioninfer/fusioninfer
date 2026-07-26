import { themes as prismThemes } from 'prism-react-renderer';
import type { Config } from '@docusaurus/types';
import type * as Preset from '@docusaurus/preset-classic';

const config: Config = {
  title: 'FusionInfer',
  tagline: 'A Kubernetes-native platform for distributed LLM inference orchestration',
  favicon: 'img/fusioninfer-logo.png',

  future: {
    v4: true,
  },

  // For GitHub Pages deployment with custom domain
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
          routeBasePath: 'blog',
          showReadingTime: true,
          blogSidebarTitle: 'All posts',
          blogSidebarCount: 'ALL',
          editUrl: 'https://github.com/fusioninfer/fusioninfer/tree/main/docs/fusioninfer/',
        },
        theme: {
          customCss: './src/css/custom.css',
        },
      } satisfies Preset.Options,
    ],
  ],

  themes: [
    '@docusaurus/theme-mermaid',
    [
      '@easyops-cn/docusaurus-search-local',
      {
        hashed: 'filename',
        language: ['en'],
        indexDocs: true,
        indexBlog: true,
        indexPages: false,
        docsRouteBasePath: '/docs',
        blogRouteBasePath: '/blog',
        highlightSearchTermsOnTargetPage: true,
        explicitSearchResultPath: true,
        searchResultLimits: 8,
        searchResultContextMaxLength: 50,
      },
    ],
  ],

  themeConfig: {
    colorMode: {
      defaultMode: 'light',
      disableSwitch: false,
      respectPrefersColorScheme: false,
    },
    tableOfContents: {
      minHeadingLevel: 2,
      maxHeadingLevel: 5,
    },
    image: 'img/fusioninfer-logo.png',
    navbar: {
      title: 'FusionInfer',
      logo: {
        alt: '',
        src: 'img/fusioninfer-logo.png',
      },
      items: [
        {
          type: 'docSidebar',
          sidebarId: 'tutorialSidebar',
          position: 'right',
          label: 'Docs',
        },
        {
          to: '/blog',
          label: 'Blogs',
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
              label: 'Overview',
              to: '/docs/intro',
            },
            {
              label: 'Deployment',
              to: '/docs/user-guide/deployment',
            },
            {
              label: 'Architecture',
              to: '/docs/design/core-design',
            },
          ],
        },
        {
          title: 'Resources',
          items: [
            {
              label: 'Blogs',
              to: '/blog',
            },
            {
              label: 'Developer Guide',
              to: '/docs/developer-guide/clientset-generation',
            },
          ],
        },
        {
          title: 'Community',
          items: [
            {
              label: 'GitHub',
              href: 'https://github.com/fusioninfer/fusioninfer',
            },
            {
              label: 'Issues',
              href: 'https://github.com/fusioninfer/fusioninfer/issues',
            },
            {
              label: 'Discussions',
              href: 'https://github.com/fusioninfer/fusioninfer/discussions',
            },
          ],
        },
      ],
      copyright: `Copyright © ${new Date().getFullYear()} FusionInfer. Built with Docusaurus.`,
    },
    prism: {
      theme: prismThemes.github,
      darkTheme: prismThemes.oceanicNext,
      additionalLanguages: ['bash', 'yaml', 'go'],
    },
  } satisfies Preset.ThemeConfig,
};

export default config;
