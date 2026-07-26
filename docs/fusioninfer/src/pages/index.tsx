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
import useBaseUrl from '@docusaurus/useBaseUrl';
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

function formatGitHubStars(count: number): string {
  return new Intl.NumberFormat('en-US', {
    notation: count >= 1000 ? 'compact' : 'standard',
    maximumFractionDigits: 1,
  })
    .format(count)
    .replace('K', 'k');
}

type Feature = {
  title: string;
  description: string;
  icon: IconType;
};

const features: Feature[] = [
  {
    title: 'One declarative API',
    description:
      'Describe the complete serving topology through a single InferenceService custom resource.',
    icon: FiLayers,
  },
  {
    title: 'Monolithic or disaggregated',
    description:
      'Run a standard worker topology or separate prefill and decode roles without changing the control plane.',
    icon: FiShuffle,
  },
  {
    title: 'Multi-node inference',
    description:
      'Coordinate distributed inference replicas with LeaderWorkerSet and explicit nodes-per-replica.',
    icon: FiServer,
  },
  {
    title: 'Inference-aware routing',
    description:
      'Choose prefix-cache, KV-cache utilization, queue-size, LoRA affinity, or P/D routing strategies.',
    icon: FiGitBranch,
  },
  {
    title: 'Gang scheduling',
    description:
      'Create Volcano PodGroups so the pods required by one distributed replica can be scheduled together.',
    icon: FiZap,
  },
  {
    title: 'Gateway API native',
    description:
      'Generate HTTPRoute and InferencePool resources, plus the Endpoint Picker deployment and its supporting configuration.',
    icon: FiCompass,
  },
];

const workflows = [
  {
    id: 'worker',
    label: 'Deploy a worker',
    title: 'Start with one serving role',
    description:
      'Declare the model container and replicas. FusionInfer reconciles the Kubernetes resources around it.',
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
              image: vllm/vllm-openai:v0.11.0
              args: ["--model", "Qwen/Qwen3-8B"]`,
  },
  {
    id: 'disaggregated',
    label: 'Split prefill / decode',
    title: 'Scale each inference phase independently',
    description:
      'Use first-class prefiller and decoder roles for a P/D-disaggregated serving topology.',
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
              image: vllm/vllm-openai:latest
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
              image: vllm/vllm-openai:latest
              args:
                - "--model"
                - "Qwen/Qwen3-8B"
                - "--kv-transfer-config"
                - '{"kv_connector":"NixlConnector","kv_role":"kv_both"}'`,
  },
  {
    id: 'routing',
    label: 'Route intelligently',
    title: 'Put inference-aware routing in front',
    description:
      'Add a router role and select a built-in strategy while retaining access to advanced EPP configuration.',
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
              image: vllm/vllm-openai:v0.11.0
              args: ["--model", "Qwen/Qwen3-8B"]`,
  },
];

const useCases = [
  {
    eyebrow: '01 / SIMPLE',
    title: 'Monolithic serving',
    description:
      'Keep the full request lifecycle in one worker role when simplicity and fast iteration matter most.',
    icon: FiBox,
    details: ['Worker replicas', 'Single-node friendly', 'OpenAI-compatible engines'],
  },
  {
    eyebrow: '02 / SPECIALIZED',
    title: 'Prefill / decode',
    description:
      'Separate prompt processing and token generation so each phase can be provisioned and scaled independently.',
    icon: FiActivity,
    details: ['Dedicated roles', 'Independent replica counts', 'P/D-aware routing'],
  },
  {
    eyebrow: '03 / DISTRIBUTED',
    title: 'Multi-node inference',
    description:
      'Run models across multiple nodes with explicit replica topology and coordinated scheduling.',
    icon: FiCpu,
    details: ['LeaderWorkerSet', 'Volcano PodGroup', 'Tensor parallel workloads'],
  },
];

const demos = [
  {
    id: 'prefix-cache',
    label: 'Prefix-cache routing',
    title: 'Route shared prefixes to the right replica',
    description:
      'See inference-aware routing reuse prefix cache state across repeated requests.',
    src: 'https://github.com/user-attachments/assets/1743bf67-2abd-42cd-a0f3-d7b65281f8cb',
    poster: '/img/demos/prefix-cache-routing-poster.jpg',
    steps: [
      'Send requests that share a prompt prefix.',
      'Observe prefix-cache-aware routing select a serving replica.',
      'Confirm the requests reach the selected worker.',
    ],
  },
  {
    id: 'multi-node',
    label: 'Multi-node inference',
    title: 'Coordinate a distributed model replica',
    description:
      'See FusionInfer manage the multi-node lifecycle behind one InferenceService.',
    src: 'https://github.com/user-attachments/assets/0c7d2126-5e71-44b7-b1ed-7ac29de7b045',
    poster: '/img/demos/multi-node-inference-poster.jpg',
    steps: [
      'Apply an InferenceService with a multi-node replica topology.',
      'Observe the distributed workload and coordinated scheduling resources.',
      'Send an inference request after the replica becomes ready.',
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
      ? formatGitHubStars(starState.count)
      : starState.status === 'loading'
        ? '…'
        : '—';
  const countLabel =
    starState.status === 'ready'
      ? `${starState.count.toLocaleString('en-US')} stars`
      : starState.status === 'loading'
        ? 'Loading star count'
        : 'Star count unavailable';

  return (
    <Link
      className={styles.githubButton}
      href={GITHUB_URL}
      aria-label={`Star FusionInfer on GitHub, ${countLabel}`}>
      <FaGithub aria-hidden="true" />
      <span className={styles.githubLabel}>Star</span>
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
  const dragonMark = useBaseUrl('/img/fusioninfer-dragon-transparent.png');

  return (
    <div className={styles.heroVisual} aria-label="FusionInfer architecture overview">
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
        <span>One API</span>
      </div>
      <div className={`${styles.heroNode} ${styles.heroNodeRight}`}>
        <FiGitBranch aria-hidden="true" />
        <span>Smart routing</span>
      </div>
      <div className={`${styles.heroNode} ${styles.heroNodeBottom}`}>
        <FiServer aria-hidden="true" />
        <span>Multi-node</span>
      </div>
      <div className={`${styles.heroNode} ${styles.heroNodeLeft}`}>
        <FiZap aria-hidden="true" />
        <span>Scheduling</span>
      </div>

      <div className={styles.dragonCore}>
        <img src={dragonMark} alt="" />
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
              The Kubernetes-native platform for LLM inference.
            </Heading>
            <p>
              Orchestrate monolithic, prefill/decode, and multi-node serving
              through one declarative API—with intelligent routing and
              scheduling built in.
            </p>
            <GitHubStarButton />
          </div>
          <HeroVisual />
        </div>

        <div className={styles.ecosystem} aria-label="FusionInfer ecosystem integrations">
          <span>Built on the Kubernetes ecosystem</span>
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
          <span>WHY FUSIONINFER</span>
          <Heading as="h2">One control plane across serving topologies</Heading>
          <p>
            Keep the user-facing API compact while FusionInfer coordinates the
            Kubernetes resources that modern inference needs.
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
          <span>BUILT FOR PLATFORM TEAMS</span>
          <Heading as="h2">A declarative workflow that stays readable</Heading>
        </div>

        <div className={styles.workflowGrid}>
          <div className={styles.workflowColumn}>
            <div
              className={styles.workflowNav}
              role="tablist"
              aria-label="Inference workflows"
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
                Read the deployment guide <FiArrowUpRight aria-hidden="true" />
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
    ['Endpoint Picker', 'Request scheduling'],
    ['LeaderWorkerSet', 'Distributed replicas'],
    ['PodGroup', 'Gang scheduling'],
  ];

  return (
    <section className={`${styles.section} ${styles.architectureSection}`}>
      <div className={styles.shell}>
        <div className={styles.architectureIntro}>
          <span>ARCHITECTURE</span>
          <Heading as="h2">A compact API over production building blocks</Heading>
          <p>
            FusionInfer watches one service definition and reconciles the
            workload, scheduling, and routing resources required by each role.
          </p>
          <Link to="/docs/design/core-design">
            Explore the architecture <FiArrowUpRight aria-hidden="true" />
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
              <small>reconcile desired topology</small>
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
          <span>SERVING TOPOLOGIES</span>
          <Heading as="h2">Start simple. Scale into the topology you need.</Heading>
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
            <span>REAL WORKLOADS</span>
            <Heading as="h2">See the control plane in motion</Heading>
          </div>
          <div className={styles.demoTabs} role="tablist" aria-label="FusionInfer demos">
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
              View the documentation <FiArrowUpRight aria-hidden="true" />
            </Link>
            <details className={styles.demoTranscript}>
              <summary>What this demo shows</summary>
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
                aria-label={`Play ${activeDemo.title}`}
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
          <span>BUILT IN THE OPEN</span>
          <Heading as="h2">Shape the next generation of LLM serving.</Heading>
          <p>
            Explore the controller, read the design decisions, and help evolve
            Kubernetes-native inference orchestration.
          </p>
        </div>
        <div className={styles.communityActions}>
          <GitHubStarButton />
          <Link to="/docs/intro">
            Read Docs <FiArrowUpRight aria-hidden="true" />
          </Link>
        </div>
      </div>
    </section>
  );
}

export default function Home(): React.JSX.Element {
  return (
    <Layout
      title="Kubernetes-native LLM inference platform"
      description="FusionInfer orchestrates monolithic, prefill/decode, and multi-node LLM inference on Kubernetes.">
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
