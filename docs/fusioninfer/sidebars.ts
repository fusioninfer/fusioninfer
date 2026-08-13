import type {SidebarsConfig} from '@docusaurus/plugin-content-docs';

const sidebars: SidebarsConfig = {
  tutorialSidebar: [
    {
      type: 'category',
      key: 'getting-started',
      label: 'Getting Started',
      items: [
        'intro',
      ],
    },
    {
      type: 'category',
      key: 'user-guide',
      label: 'User Guide',
      items: [
        'user-guide/deployment',
      ],
    },
    {
      type: 'category',
      key: 'design',
      label: 'Design',
      items: [
        'design/model-serving',
        'design/model',
        'design/runtime-profile',
        'design/inference-deployment',
        'design/workload-orchestration',
        'design/core-design',
        'design/router',
      ],
    },
    {
      type: 'category',
      key: 'developer-guide',
      label: 'Developer Guide',
      items: [
        'developer-guide/clientset-generation',
      ],
    },
  ],
};

export default sidebars;
