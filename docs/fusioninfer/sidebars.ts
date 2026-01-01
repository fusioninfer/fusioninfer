import type {SidebarsConfig} from '@docusaurus/plugin-content-docs';

const sidebars: SidebarsConfig = {
  tutorialSidebar: [
    'intro',
    {
      type: 'category',
      label: 'User Guide',
      items: [
        'user-guide/deployment',
      ],
    },
    {
      type: 'category',
      label: 'Design',
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
