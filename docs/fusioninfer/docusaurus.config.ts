import { themes as prismThemes } from 'prism-react-renderer';
import type { Config } from '@docusaurus/types';
import type * as Preset from '@docusaurus/preset-classic';

const docsEditUrl = 'https://github.com/fusioninfer/fusioninfer/edit/main/docs/fusioninfer/';

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
    path: 'i18n',
    defaultLocale: 'en',
    locales: ['en', 'zh'],
    localeConfigs: {
      en: {
        label: 'English',
        htmlLang: 'en',
      },
      zh: {
        path: 'zh-Hans',
        label: '简体中文',
        htmlLang: 'zh-CN',
      },
    },
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
          editUrl: docsEditUrl,
          editLocalizedFiles: true,
        },
        blog: {
          routeBasePath: 'blog',
          showReadingTime: true,
          blogSidebarTitle: 'All posts',
          blogSidebarCount: 'ALL',
          editUrl: docsEditUrl,
          editLocalizedFiles: true,
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
        language: ['en', 'zh'],
        indexDocs: true,
        indexBlog: true,
        indexPages: false,
        docsDir: [
          'docs',
          'i18n/zh-Hans/docusaurus-plugin-content-docs/current',
        ],
        blogDir: [
          'blog',
          'i18n/zh-Hans/docusaurus-plugin-content-blog',
        ],
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
        {
          type: 'localeDropdown',
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
              label: 'Design',
              to: '/docs/design/model-serving',
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
