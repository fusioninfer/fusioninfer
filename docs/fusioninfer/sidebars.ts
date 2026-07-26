import type {SidebarsConfig} from '@docusaurus/plugin-content-docs';

const sidebars: SidebarsConfig = {
  tutorialSidebar: [
    {
      type: 'category',
      label: 'Getting Started',
      items: [
        'intro',
      ],
    },
    {
      type: 'category',
      label: 'User Guide',
      items: [
        'user-guide/deployment',
      ],
    },
    {
      type: 'category',
      label: 'Architecture',
      items: [
        'design/core-design',
        'design/router',
      ],
    },
    {
      type: 'category',
      label: 'Developer Guide',
      items: [
        'developer-guide/clientset-generation',
      ],
    },
  ],
};

export default sidebars;
