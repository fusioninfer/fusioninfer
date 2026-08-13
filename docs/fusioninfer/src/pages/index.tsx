import {useEffect, useState} from 'react';
import type {IconType} from 'react-icons';
import {
  FiActivity,
  FiArrowDown,
  FiArrowUpRight,
  FiBox,
  FiCheck,
  FiCompass,
  FiCpu,
  FiGitBranch,
  FiLayers,
  FiPlay,
  FiServer,
  FiShuffle,
  FiZap,
} from 'react-icons/fi';
import {FaGithub} from 'react-icons/fa6';
import Link from '@docusaurus/Link';
import Translate, {translate} from '@docusaurus/Translate';
import useBaseUrl from '@docusaurus/useBaseUrl';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import Layout from '@theme/Layout';
import Heading from '@theme/Heading';
import styles from './index.module.css';

const GITHUB_URL = 'https://github.com/fusioninfer/fusioninfer';
const GITHUB_REPOSITORY_API =
  'https://api.github.com/repos/fusioninfer/fusioninfer';

type GitHubStarState =
  | {status: 'loading'}
  | {status: 'ready'; count: number}
  | {status: 'error'};

let githubStarRequest: Promise<number> | undefined;

function requestGitHubStars(): Promise<number> {
  githubStarRequest ??= fetch(GITHUB_REPOSITORY_API, {
    headers: {
      Accept: 'application/vnd.github+json',
    },
  }).then(async (response) => {
    if (!response.ok) {
      throw new Error(`GitHub star request failed: ${response.status}`);
    }

    const data = (await response.json()) as {stargazers_count?: unknown};
    if (typeof data.stargazers_count !== 'number') {
      throw new Error('GitHub star response is missing stargazers_count');
    }

    return data.stargazers_count;
  });

  return githubStarRequest;
}

function formatGitHubStars(count: number, locale: string): string {
  const formattedCount = new Intl.NumberFormat(locale, {
    notation: count >= 1000 ? 'compact' : 'standard',
    maximumFractionDigits: 1,
  }).format(count);

  return locale.toLowerCase().startsWith('en')
    ? formattedCount.replace('K', 'k')
    : formattedCount;
}

type Feature = {
  title: string;
  description: string;
  icon: IconType;
};

const features: Feature[] = [
  {
    title: translate({
      id: 'homepage.features.oneApi.title',
      message: 'One declarative API',
      description: 'Title of the declarative API homepage feature',
    }),
    description: translate({
      id: 'homepage.features.oneApi.description',
      message:
        'Describe the complete serving topology through a single InferenceService custom resource.',
      description: 'Description of the declarative API homepage feature',
    }),
    icon: FiLayers,
  },
  {
    title: translate({
      id: 'homepage.features.topologies.title',
      message: 'Monolithic or disaggregated',
      description: 'Title of the serving topologies homepage feature',
    }),
    description: translate({
      id: 'homepage.features.topologies.description',
      message:
        'Run a standard worker topology or separate prefill and decode roles without changing the control plane.',
      description: 'Description of the serving topologies homepage feature',
    }),
    icon: FiShuffle,
  },
  {
    title: translate({
      id: 'homepage.features.multiNode.title',
      message: 'Multi-node inference',
      description: 'Title of the multi-node inference homepage feature',
    }),
    description: translate({
      id: 'homepage.features.multiNode.description',
      message:
        'Coordinate distributed inference replicas with LeaderWorkerSet and explicit nodes-per-replica.',
      description: 'Description of the multi-node inference homepage feature',
    }),
    icon: FiServer,
  },
  {
    title: translate({
      id: 'homepage.features.routing.title',
      message: 'Inference-aware routing',
      description: 'Title of the inference-aware routing homepage feature',
    }),
    description: translate({
      id: 'homepage.features.routing.description',
      message:
        'Choose prefix-cache, KV-cache utilization, queue-size, LoRA affinity, or P/D routing strategies.',
      description: 'Description of the inference-aware routing homepage feature',
    }),
    icon: FiGitBranch,
  },
  {
    title: translate({
      id: 'homepage.features.gangScheduling.title',
      message: 'Gang scheduling',
      description: 'Title of the gang scheduling homepage feature',
    }),
    description: translate({
      id: 'homepage.features.gangScheduling.description',
      message:
        'Create Volcano PodGroups so the pods required by one distributed replica can be scheduled together.',
      description: 'Description of the gang scheduling homepage feature',
    }),
    icon: FiZap,
  },
  {
    title: translate({
      id: 'homepage.features.gatewayApi.title',
      message: 'Gateway API native',
      description: 'Title of the Gateway API homepage feature',
    }),
    description: translate({
      id: 'homepage.features.gatewayApi.description',
      message:
        'Generate HTTPRoute and InferencePool resources, plus the Endpoint Picker deployment and its supporting configuration.',
      description: 'Description of the Gateway API homepage feature',
    }),
    icon: FiCompass,
  },
];

const workflows = [
  {
    id: 'worker',
    label: translate({
      id: 'homepage.workflows.worker.label',
      message: 'Deploy a worker',
      description: 'Tab label for the single-worker deployment workflow',
    }),
    title: translate({
      id: 'homepage.workflows.worker.title',
      message: 'Start with one serving role',
      description: 'Title of the single-worker deployment workflow',
    }),
    description: translate({
      id: 'homepage.workflows.worker.description',
      message:
        'Declare the model container and replicas. FusionInfer reconciles the Kubernetes resources around it.',
      description: 'Description of the single-worker deployment workflow',
    }),
    code: `apiVersion: fusioninfer.io/v1alpha1
kind: InferenceService
metadata:
  name: qwen-inference
spec:
  roles:
    - name: inference
      componentType: worker
      replicas: 2
      template:
        spec:
          containers:
            - name: vllm
              image: vllm/vllm-openai:v0.26.0
              args: ["--model", "Qwen/Qwen3-8B"]`,
  },
  {
    id: 'disaggregated',
    label: translate({
      id: 'homepage.workflows.disaggregated.label',
      message: 'Split prefill / decode',
      description: 'Tab label for the disaggregated serving workflow',
    }),
    title: translate({
      id: 'homepage.workflows.disaggregated.title',
      message: 'Scale each inference phase independently',
      description: 'Title of the disaggregated serving workflow',
    }),
    description: translate({
      id: 'homepage.workflows.disaggregated.description',
      message:
        'Use first-class prefiller and decoder roles for a P/D-disaggregated serving topology.',
      description: 'Description of the disaggregated serving workflow',
    }),
    code: `apiVersion: fusioninfer.io/v1alpha1
kind: InferenceService
metadata:
  name: qwen-disaggregated
spec:
  roles:
    - name: prefill
      componentType: prefiller
      replicas: 2
      template:
        spec:
          containers:
            - name: vllm-prefill
              image: vllm/vllm-openai:v0.26.0
              args:
                - "--model"
                - "Qwen/Qwen3-8B"
                - "--kv-transfer-config"
                - '{"kv_connector":"NixlConnector","kv_role":"kv_both"}'
    - name: decode
      componentType: decoder
      replicas: 4
      template:
        spec:
          containers:
            - name: vllm-decode
              image: vllm/vllm-openai:v0.26.0
              args:
                - "--model"
                - "Qwen/Qwen3-8B"
                - "--kv-transfer-config"
                - '{"kv_connector":"NixlConnector","kv_role":"kv_both"}'`,
  },
  {
    id: 'routing',
    label: translate({
      id: 'homepage.workflows.routing.label',
      message: 'Route intelligently',
      description: 'Tab label for the inference-aware routing workflow',
    }),
    title: translate({
      id: 'homepage.workflows.routing.title',
      message: 'Put inference-aware routing in front',
      description: 'Title of the inference-aware routing workflow',
    }),
    description: translate({
      id: 'homepage.workflows.routing.description',
      message:
        'Add a router role and select a built-in strategy while retaining access to advanced EPP configuration.',
      description: 'Description of the inference-aware routing workflow',
    }),
    code: `apiVersion: fusioninfer.io/v1alpha1
kind: InferenceService
metadata:
  name: qwen-routed
spec:
  roles:
    - name: router
      componentType: router
      strategy: prefix-cache
      httproute:
        parentRefs:
          - name: inference-gateway
    - name: inference
      componentType: worker
      replicas: 3
      template:
        spec:
          containers:
            - name: vllm
              image: vllm/vllm-openai:v0.26.0
              args: ["--model", "Qwen/Qwen3-8B"]`,
  },
];

const useCases = [
  {
    eyebrow: translate({
      id: 'homepage.useCases.monolithic.eyebrow',
      message: '01 / SIMPLE',
      description: 'Category label for the monolithic serving use case',
    }),
    title: translate({
      id: 'homepage.useCases.monolithic.title',
      message: 'Monolithic serving',
      description: 'Title of the monolithic serving use case',
    }),
    description: translate({
      id: 'homepage.useCases.monolithic.description',
      message:
        'Keep the full request lifecycle in one worker role when simplicity and fast iteration matter most.',
      description: 'Description of the monolithic serving use case',
    }),
    icon: FiBox,
    details: [
      translate({
        id: 'homepage.useCases.monolithic.details.workerReplicas',
        message: 'Worker replicas',
        description: 'Worker replica detail for the monolithic serving use case',
      }),
      translate({
        id: 'homepage.useCases.monolithic.details.singleNode',
        message: 'Single-node friendly',
        description: 'Single-node detail for the monolithic serving use case',
      }),
      translate({
        id: 'homepage.useCases.monolithic.details.openAi',
        message: 'OpenAI-compatible engines',
        description: 'Engine compatibility detail for the monolithic serving use case',
      }),
    ],
  },
  {
    eyebrow: translate({
      id: 'homepage.useCases.disaggregated.eyebrow',
      message: '02 / SPECIALIZED',
      description: 'Category label for the prefill/decode use case',
    }),
    title: translate({
      id: 'homepage.useCases.disaggregated.title',
      message: 'Prefill / decode',
      description: 'Title of the prefill/decode use case',
    }),
    description: translate({
      id: 'homepage.useCases.disaggregated.description',
      message:
        'Separate prompt processing and token generation so each phase can be provisioned and scaled independently.',
      description: 'Description of the prefill/decode use case',
    }),
    icon: FiActivity,
    details: [
      translate({
        id: 'homepage.useCases.disaggregated.details.roles',
        message: 'Dedicated roles',
        description: 'Role detail for the prefill/decode use case',
      }),
      translate({
        id: 'homepage.useCases.disaggregated.details.replicas',
        message: 'Independent replica counts',
        description: 'Replica count detail for the prefill/decode use case',
      }),
      translate({
        id: 'homepage.useCases.disaggregated.details.routing',
        message: 'P/D-aware routing',
        description: 'Routing detail for the prefill/decode use case',
      }),
    ],
  },
  {
    eyebrow: translate({
      id: 'homepage.useCases.multiNode.eyebrow',
      message: '03 / DISTRIBUTED',
      description: 'Category label for the multi-node inference use case',
    }),
    title: translate({
      id: 'homepage.useCases.multiNode.title',
      message: 'Multi-node inference',
      description: 'Title of the multi-node inference use case',
    }),
    description: translate({
      id: 'homepage.useCases.multiNode.description',
      message:
        'Run models across multiple nodes with explicit replica topology and coordinated scheduling.',
      description: 'Description of the multi-node inference use case',
    }),
    icon: FiCpu,
    details: [
      'LeaderWorkerSet',
      'Volcano PodGroup',
      translate({
        id: 'homepage.useCases.multiNode.details.tensorParallel',
        message: 'Tensor parallel workloads',
        description: 'Tensor parallel detail for the multi-node inference use case',
      }),
    ],
  },
];

const demos = [
  {
    id: 'prefix-cache',
    label: translate({
      id: 'homepage.demos.prefixCache.label',
      message: 'Prefix-cache routing',
      description: 'Tab label for the prefix-cache routing demo',
    }),
    title: translate({
      id: 'homepage.demos.prefixCache.title',
      message: 'Route shared prefixes to the right replica',
      description: 'Title of the prefix-cache routing demo',
    }),
    description: translate({
      id: 'homepage.demos.prefixCache.description',
      message:
        'See inference-aware routing reuse prefix cache state across repeated requests.',
      description: 'Description of the prefix-cache routing demo',
    }),
    src: 'https://github.com/user-attachments/assets/1743bf67-2abd-42cd-a0f3-d7b65281f8cb',
    poster: '/img/demos/prefix-cache-routing-poster.jpg',
    steps: [
      translate({
        id: 'homepage.demos.prefixCache.steps.send',
        message: 'Send requests that share a prompt prefix.',
        description: 'First transcript step of the prefix-cache routing demo',
      }),
      translate({
        id: 'homepage.demos.prefixCache.steps.observe',
        message: 'Observe prefix-cache-aware routing select a serving replica.',
        description: 'Second transcript step of the prefix-cache routing demo',
      }),
      translate({
        id: 'homepage.demos.prefixCache.steps.confirm',
        message: 'Confirm the requests reach the selected worker.',
        description: 'Third transcript step of the prefix-cache routing demo',
      }),
    ],
  },
  {
    id: 'multi-node',
    label: translate({
      id: 'homepage.demos.multiNode.label',
      message: 'Multi-node inference',
      description: 'Tab label for the multi-node inference demo',
    }),
    title: translate({
      id: 'homepage.demos.multiNode.title',
      message: 'Coordinate a distributed model replica',
      description: 'Title of the multi-node inference demo',
    }),
    description: translate({
      id: 'homepage.demos.multiNode.description',
      message:
        'See FusionInfer manage the multi-node lifecycle behind one InferenceService.',
      description: 'Description of the multi-node inference demo',
    }),
    src: 'https://github.com/user-attachments/assets/0c7d2126-5e71-44b7-b1ed-7ac29de7b045',
    poster: '/img/demos/multi-node-inference-poster.jpg',
    steps: [
      translate({
        id: 'homepage.demos.multiNode.steps.apply',
        message: 'Apply an InferenceService with a multi-node replica topology.',
        description: 'First transcript step of the multi-node inference demo',
      }),
      translate({
        id: 'homepage.demos.multiNode.steps.observe',
        message:
          'Observe the distributed workload and coordinated scheduling resources.',
        description: 'Second transcript step of the multi-node inference demo',
      }),
      translate({
        id: 'homepage.demos.multiNode.steps.send',
        message: 'Send an inference request after the replica becomes ready.',
        description: 'Third transcript step of the multi-node inference demo',
      }),
    ],
  },
];

function handleTabKeyDown(
  event: React.KeyboardEvent<HTMLButtonElement>,
  ids: string[],
  activeId: string,
  onSelect: (id: string) => void,
  idPrefix: string,
  orientation: 'horizontal' | 'vertical',
) {
  const previousKey = orientation === 'horizontal' ? 'ArrowLeft' : 'ArrowUp';
  const nextKey = orientation === 'horizontal' ? 'ArrowRight' : 'ArrowDown';
  const currentIndex = ids.indexOf(activeId);
  let nextIndex: number | undefined;

  if (event.key === previousKey) {
    nextIndex = (currentIndex - 1 + ids.length) % ids.length;
  } else if (event.key === nextKey) {
    nextIndex = (currentIndex + 1) % ids.length;
  } else if (event.key === 'Home') {
    nextIndex = 0;
  } else if (event.key === 'End') {
    nextIndex = ids.length - 1;
  }

  if (nextIndex === undefined) {
    return;
  }

  event.preventDefault();
  const nextId = ids[nextIndex];
  onSelect(nextId);
  document.getElementById(`${idPrefix}-${nextId}`)?.focus();
}

function GitHubStarButton() {
  const {
    i18n: {currentLocale},
  } = useDocusaurusContext();
  const [starState, setStarState] = useState<GitHubStarState>({
    status: 'loading',
  });

  useEffect(() => {
    let active = true;

    requestGitHubStars()
      .then((count) => {
        if (active) {
          setStarState({status: 'ready', count});
        }
      })
      .catch(() => {
        if (active) {
          setStarState({status: 'error'});
        }
      });

    return () => {
      active = false;
    };
  }, []);

  const count =
    starState.status === 'ready'
      ? formatGitHubStars(starState.count, currentLocale)
      : starState.status === 'loading'
        ? '…'
        : '—';
  const countLabel =
    starState.status === 'ready'
      ? translate(
          {
            id: 'homepage.githubStars.readyLabel',
            message: '{count} stars',
            description: 'Accessible GitHub star count on the homepage',
          },
          {count: starState.count.toLocaleString(currentLocale)},
        )
      : starState.status === 'loading'
        ? translate({
            id: 'homepage.githubStars.loadingLabel',
            message: 'Loading star count',
            description: 'Accessible loading state for the GitHub star count',
          })
        : translate({
            id: 'homepage.githubStars.errorLabel',
            message: 'Star count unavailable',
            description: 'Accessible error state for the GitHub star count',
          });

  return (
    <Link
      className={styles.githubButton}
      href={GITHUB_URL}
      aria-label={translate(
        {
          id: 'homepage.githubStars.buttonAriaLabel',
          message: 'Star FusionInfer on GitHub, {countLabel}',
          description: 'Accessible label for the homepage GitHub star button',
        },
        {countLabel},
      )}>
      <FaGithub aria-hidden="true" />
      <span className={styles.githubLabel}>
        <Translate
          id="homepage.githubStars.buttonLabel"
          description="Visible label for the homepage GitHub star button">
          Star
        </Translate>
      </span>
      <span
        aria-busy={starState.status === 'loading'}
        aria-label={countLabel}
        aria-live="polite"
        className={styles.githubCount}
        data-status={starState.status}>
        {count}
      </span>
    </Link>
  );
}

function HeroVisual() {
  const brandMark = useBaseUrl('/img/fusioninfer-logo.png');

  return (
    <div
      className={styles.heroVisual}
      aria-label={translate({
        id: 'homepage.hero.visualAriaLabel',
        message: 'FusionInfer architecture overview',
        description: 'Accessible label for the homepage hero architecture visual',
      })}>
      <div className={styles.heroGlow} aria-hidden="true" />
      <svg
        className={styles.orbitLines}
        viewBox="0 0 560 500"
        role="presentation">
        <circle cx="280" cy="250" r="182" />
        <circle cx="280" cy="250" r="132" />
        <path d="M90 204C140 94 252 44 370 80" />
        <path d="M468 284C430 402 306 458 190 414" />
      </svg>

      <span className={`${styles.orbitDot} ${styles.orbitDotOne}`} aria-hidden="true" />
      <span className={`${styles.orbitDot} ${styles.orbitDotTwo}`} aria-hidden="true" />
      <span className={`${styles.orbitDot} ${styles.orbitDotThree}`} aria-hidden="true" />

      <div className={`${styles.heroNode} ${styles.heroNodeTop}`}>
        <FiLayers aria-hidden="true" />
        <span>
          <Translate
            id="homepage.hero.nodes.oneApi"
            description="Label for the API node in the homepage hero visual">
            One API
          </Translate>
        </span>
      </div>
      <div className={`${styles.heroNode} ${styles.heroNodeRight}`}>
        <FiGitBranch aria-hidden="true" />
        <span>
          <Translate
            id="homepage.hero.nodes.smartRouting"
            description="Label for the routing node in the homepage hero visual">
            Smart routing
          </Translate>
        </span>
      </div>
      <div className={`${styles.heroNode} ${styles.heroNodeBottom}`}>
        <FiServer aria-hidden="true" />
        <span>
          <Translate
            id="homepage.hero.nodes.multiNode"
            description="Label for the multi-node node in the homepage hero visual">
            Multi-node
          </Translate>
        </span>
      </div>
      <div className={`${styles.heroNode} ${styles.heroNodeLeft}`}>
        <FiZap aria-hidden="true" />
        <span>
          <Translate
            id="homepage.hero.nodes.scheduling"
            description="Label for the scheduling node in the homepage hero visual">
            Scheduling
          </Translate>
        </span>
      </div>

      <div className={styles.dragonCore}>
        <img src={brandMark} alt="" />
        <div className={styles.coreLabel}>
          <strong>InferenceService</strong>
          <span>fusioninfer.io/v1alpha1</span>
        </div>
      </div>
    </div>
  );
}

function Hero() {
  return (
    <header className={styles.hero}>
      <div className={styles.shell}>
        <div className={styles.heroGrid}>
          <div className={styles.heroCopy}>
            <Heading as="h1">
              <Translate
                id="homepage.hero.title"
                description="Primary heading on the FusionInfer homepage">
                The Kubernetes-native platform for LLM inference.
              </Translate>
            </Heading>
            <p>
              <Translate
                id="homepage.hero.description"
                description="Primary description on the FusionInfer homepage">
                {
                  'Orchestrate monolithic, prefill/decode, and multi-node serving through one declarative API—with intelligent routing and scheduling built in.'
                }
              </Translate>
            </p>
            <GitHubStarButton />
          </div>
          <HeroVisual />
        </div>

        <div
          className={styles.ecosystem}
          aria-label={translate({
            id: 'homepage.hero.ecosystemAriaLabel',
            message: 'FusionInfer ecosystem integrations',
            description: 'Accessible label for the homepage ecosystem integrations',
          })}>
          <span>
            <Translate
              id="homepage.hero.ecosystemLabel"
              description="Introductory label for homepage ecosystem integrations">
              Built on the Kubernetes ecosystem
            </Translate>
          </span>
          <ul>
            <li>Gateway API</li>
            <li>Inference Extension</li>
            <li>LeaderWorkerSet</li>
            <li>Volcano</li>
            <li>Endpoint Picker</li>
          </ul>
        </div>
      </div>
    </header>
  );
}

function FeatureGrid() {
  return (
    <section className={styles.section}>
      <div className={styles.shell}>
        <div className={styles.sectionHeading}>
          <span>
            <Translate
              id="homepage.features.eyebrow"
              description="Eyebrow label for the homepage features section">
              WHY FUSIONINFER
            </Translate>
          </span>
          <Heading as="h2">
            <Translate
              id="homepage.features.heading"
              description="Heading for the homepage features section">
              One control plane across serving topologies
            </Translate>
          </Heading>
          <p>
            <Translate
              id="homepage.features.intro"
              description="Introductory text for the homepage features section">
              {
                'Keep the user-facing API compact while FusionInfer coordinates the Kubernetes resources that modern inference needs.'
              }
            </Translate>
          </p>
        </div>

        <div className={styles.featureGrid}>
          {features.map(({title, description, icon: Icon}) => (
            <article className={styles.featureCard} key={title}>
              <Icon aria-hidden="true" />
              <Heading as="h3">{title}</Heading>
              <p>{description}</p>
            </article>
          ))}
        </div>
      </div>
    </section>
  );
}

function WorkflowSection() {
  const [activeId, setActiveId] = useState(workflows[0].id);
  const workflowIds = workflows.map((workflow) => workflow.id);
  const activeWorkflow = workflows.find((workflow) => workflow.id === activeId)!;

  return (
    <section className={`${styles.section} ${styles.workflowSection}`}>
      <div className={styles.shell}>
        <div className={styles.sectionHeading}>
          <span>
            <Translate
              id="homepage.workflows.eyebrow"
              description="Eyebrow label for the homepage workflows section">
              BUILT FOR PLATFORM TEAMS
            </Translate>
          </span>
          <Heading as="h2">
            <Translate
              id="homepage.workflows.heading"
              description="Heading for the homepage workflows section">
              A declarative workflow that stays readable
            </Translate>
          </Heading>
        </div>

        <div className={styles.workflowGrid}>
          <div className={styles.workflowColumn}>
            <div
              className={styles.workflowNav}
              role="tablist"
              aria-label={translate({
                id: 'homepage.workflows.tabListAriaLabel',
                message: 'Inference workflows',
                description: 'Accessible label for the homepage workflow tabs',
              })}
              aria-orientation="vertical">
              {workflows.map((workflow) => (
                <button
                  id={`workflow-tab-${workflow.id}`}
                  key={workflow.id}
                  type="button"
                  role="tab"
                  aria-controls="workflow-panel"
                  aria-selected={activeId === workflow.id}
                  tabIndex={activeId === workflow.id ? 0 : -1}
                  className={activeId === workflow.id ? styles.activeWorkflow : undefined}
                  onClick={() => setActiveId(workflow.id)}
                  onKeyDown={(event) =>
                    handleTabKeyDown(
                      event,
                      workflowIds,
                      activeId,
                      setActiveId,
                      'workflow-tab',
                      'vertical',
                    )
                  }>
                  <span>{workflow.label}</span>
                  <FiArrowUpRight aria-hidden="true" />
                </button>
              ))}
            </div>
            <div className={styles.workflowSummary}>
              <Heading as="h3">{activeWorkflow.title}</Heading>
              <p>{activeWorkflow.description}</p>
              <Link to="/docs/user-guide/deployment">
                <Translate
                  id="homepage.workflows.deploymentGuideCta"
                  description="Link label for the deployment guide from the workflows section">
                  Read the deployment guide
                </Translate>{' '}
                <FiArrowUpRight aria-hidden="true" />
              </Link>
            </div>
          </div>

          <div
            id="workflow-panel"
            className={styles.codePanel}
            role="tabpanel"
            tabIndex={0}
            aria-labelledby={`workflow-tab-${activeId}`}>
            <div className={styles.codePanelBar}>
              <span />
              <span />
              <span />
              <code>inference-service.yaml</code>
            </div>
            <pre>
              <code>{activeWorkflow.code}</code>
            </pre>
          </div>
        </div>
      </div>
    </section>
  );
}

function ArchitectureSection() {
  const outputs = [
    ['HTTPRoute', 'Gateway API'],
    ['InferencePool', 'Inference Extension'],
    [
      'Endpoint Picker',
      translate({
        id: 'homepage.architecture.outputs.requestScheduling',
        message: 'Request scheduling',
        description: 'Purpose of Endpoint Picker in the architecture diagram',
      }),
    ],
    [
      'LeaderWorkerSet',
      translate({
        id: 'homepage.architecture.outputs.distributedReplicas',
        message: 'Distributed replicas',
        description: 'Purpose of LeaderWorkerSet in the architecture diagram',
      }),
    ],
    [
      'PodGroup',
      translate({
        id: 'homepage.architecture.outputs.gangScheduling',
        message: 'Gang scheduling',
        description: 'Purpose of PodGroup in the architecture diagram',
      }),
    ],
  ];

  return (
    <section className={`${styles.section} ${styles.architectureSection}`}>
      <div className={styles.shell}>
        <div className={styles.architectureIntro}>
          <span>
            <Translate
              id="homepage.architecture.eyebrow"
              description="Eyebrow label for the homepage architecture section">
              ARCHITECTURE
            </Translate>
          </span>
          <Heading as="h2">
            <Translate
              id="homepage.architecture.heading"
              description="Heading for the homepage architecture section">
              A compact API over production building blocks
            </Translate>
          </Heading>
          <p>
            <Translate
              id="homepage.architecture.description"
              description="Description of the FusionInfer architecture on the homepage">
              {
                'FusionInfer binds explicit Model, RuntimeProfile, and InferenceDeployment resources, then reconciles model downloading and caching, workloads, scheduling, and routing.'
              }
            </Translate>
          </p>
          <Link to="/docs/design/model-serving">
            <Translate
              id="homepage.architecture.designCta"
              description="Link label for the design documentation from the architecture section">
              Explore the design
            </Translate>{' '}
            <FiArrowUpRight aria-hidden="true" />
          </Link>
        </div>

        <div className={styles.architectureDiagram}>
          <div className={styles.architectureInput}>
            <span>fusioninfer.io/v1alpha1</span>
            <strong>InferenceService</strong>
            <small>roles · replicas · multinode · strategy</small>
          </div>
          <FiArrowDown className={styles.architectureArrow} aria-hidden="true" />
          <div className={styles.controllerNode}>
            <FiCpu aria-hidden="true" />
            <span>
              <strong>FusionInfer Controller</strong>
              <small>
                <Translate
                  id="homepage.architecture.controllerPurpose"
                  description="Controller purpose in the homepage architecture diagram">
                  reconcile desired topology
                </Translate>
              </small>
            </span>
          </div>
          <FiArrowDown className={styles.architectureArrow} aria-hidden="true" />
          <div className={styles.architectureOutputs}>
            {outputs.map(([name, purpose]) => (
              <div key={name}>
                <FiCheck aria-hidden="true" />
                <span>
                  <strong>{name}</strong>
                  <small>{purpose}</small>
                </span>
              </div>
            ))}
          </div>
        </div>
      </div>
    </section>
  );
}

function UseCasesSection() {
  return (
    <section className={styles.section}>
      <div className={styles.shell}>
        <div className={styles.sectionHeading}>
          <span>
            <Translate
              id="homepage.useCases.eyebrow"
              description="Eyebrow label for the homepage serving use cases section">
              SERVING TOPOLOGIES
            </Translate>
          </span>
          <Heading as="h2">
            <Translate
              id="homepage.useCases.heading"
              description="Heading for the homepage serving use cases section">
              Start simple. Scale into the topology you need.
            </Translate>
          </Heading>
        </div>

        <div className={styles.useCaseGrid}>
          {useCases.map(({eyebrow, title, description, icon: Icon, details}) => (
            <article className={styles.useCaseCard} key={title}>
              <div className={styles.useCaseTopline}>
                <span>{eyebrow}</span>
                <Icon aria-hidden="true" />
              </div>
              <Heading as="h3">{title}</Heading>
              <p>{description}</p>
              <ul>
                {details.map((detail) => (
                  <li key={detail}>
                    <FiCheck aria-hidden="true" />
                    {detail}
                  </li>
                ))}
              </ul>
            </article>
          ))}
        </div>
      </div>
    </section>
  );
}

function DemoSection() {
  const [activeId, setActiveId] = useState(demos[0].id);
  const [isPlaying, setIsPlaying] = useState(false);
  const demoIds = demos.map((demo) => demo.id);
  const activeDemo = demos.find((demo) => demo.id === activeId)!;
  const activePoster = useBaseUrl(activeDemo.poster);
  const selectDemo = (id: string) => {
    setActiveId(id);
    setIsPlaying(false);
  };

  return (
    <section className={`${styles.section} ${styles.demoSection}`}>
      <div className={styles.shell}>
        <div className={styles.demoHeader}>
          <div>
            <span>
              <Translate
                id="homepage.demos.eyebrow"
                description="Eyebrow label for the homepage demos section">
                REAL WORKLOADS
              </Translate>
            </span>
            <Heading as="h2">
              <Translate
                id="homepage.demos.heading"
                description="Heading for the homepage demos section">
                See the control plane in motion
              </Translate>
            </Heading>
          </div>
          <div
            className={styles.demoTabs}
            role="tablist"
            aria-label={translate({
              id: 'homepage.demos.tabListAriaLabel',
              message: 'FusionInfer demos',
              description: 'Accessible label for the homepage demo tabs',
            })}>
            {demos.map((demo) => (
              <button
                id={`demo-tab-${demo.id}`}
                key={demo.id}
                type="button"
                role="tab"
                aria-controls="demo-panel"
                aria-selected={activeId === demo.id}
                tabIndex={activeId === demo.id ? 0 : -1}
                onClick={() => selectDemo(demo.id)}
                onKeyDown={(event) =>
                  handleTabKeyDown(
                    event,
                    demoIds,
                    activeId,
                    selectDemo,
                    'demo-tab',
                    'horizontal',
                  )
                }>
                {demo.label}
              </button>
            ))}
          </div>
        </div>

        <div className={styles.demoGrid}>
          <div className={styles.demoCopy}>
            <Heading as="h3">{activeDemo.title}</Heading>
            <p>{activeDemo.description}</p>
            <Link to="/docs/intro">
              <Translate
                id="homepage.demos.documentationCta"
                description="Link label for documentation from the demos section">
                View the documentation
              </Translate>{' '}
              <FiArrowUpRight aria-hidden="true" />
            </Link>
            <details className={styles.demoTranscript}>
              <summary>
                <Translate
                  id="homepage.demos.transcriptSummary"
                  description="Summary label for a homepage demo transcript">
                  What this demo shows
                </Translate>
              </summary>
              <ol>
                {activeDemo.steps.map((step) => (
                  <li key={step}>{step}</li>
                ))}
              </ol>
            </details>
          </div>
          <div
            id="demo-panel"
            className={styles.videoFrame}
            role="tabpanel"
            tabIndex={0}
            aria-labelledby={`demo-tab-${activeId}`}>
            {isPlaying ? (
              <video
                key={activeDemo.id}
                autoPlay
                controls
                playsInline
                preload="metadata"
                poster={activePoster}
                aria-label={activeDemo.title}>
                <source src={activeDemo.src} />
              </video>
            ) : (
              <button
                type="button"
                className={styles.posterButton}
                aria-label={translate(
                  {
                    id: 'homepage.demos.playAriaLabel',
                    message: 'Play {title}',
                    description: 'Accessible label for a homepage demo play button',
                  },
                  {title: activeDemo.title},
                )}
                onClick={() => setIsPlaying(true)}>
                <img src={activePoster} alt="" />
                <span>
                  <FiPlay aria-hidden="true" />
                </span>
              </button>
            )}
          </div>
        </div>
      </div>
    </section>
  );
}

function CommunitySection() {
  return (
    <section className={styles.communitySection}>
      <div className={styles.shell}>
        <div>
          <span>
            <Translate
              id="homepage.community.eyebrow"
              description="Eyebrow label for the homepage community section">
              BUILT IN THE OPEN
            </Translate>
          </span>
          <Heading as="h2">
            <Translate
              id="homepage.community.heading"
              description="Heading for the homepage community section">
              Shape the next generation of LLM serving.
            </Translate>
          </Heading>
          <p>
            <Translate
              id="homepage.community.description"
              description="Description of the homepage community section">
              {
                'Explore the controller, read the design decisions, and help evolve Kubernetes-native inference orchestration.'
              }
            </Translate>
          </p>
        </div>
        <div className={styles.communityActions}>
          <GitHubStarButton />
          <Link to="/docs/intro">
            <Translate
              id="homepage.community.documentationCta"
              description="Link label for documentation from the community section">
              Read Docs
            </Translate>{' '}
            <FiArrowUpRight aria-hidden="true" />
          </Link>
        </div>
      </div>
    </section>
  );
}

export default function Home(): React.JSX.Element {
  return (
    <Layout
      title={translate({
        id: 'homepage.seo.title',
        message: 'Kubernetes-native LLM inference platform',
        description: 'SEO title for the FusionInfer homepage',
      })}
      description={translate({
        id: 'homepage.seo.description',
        message:
          'FusionInfer orchestrates monolithic, prefill/decode, and multi-node LLM inference on Kubernetes.',
        description: 'SEO description for the FusionInfer homepage',
      })}>
      <main className={styles.homeMain}>
        <Hero />
        <FeatureGrid />
        <WorkflowSection />
        <ArchitectureSection />
        <UseCasesSection />
        <DemoSection />
        <CommunitySection />
      </main>
    </Layout>
  );
}
