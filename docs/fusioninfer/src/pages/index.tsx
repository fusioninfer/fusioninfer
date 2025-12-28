import clsx from 'clsx';
import Link from '@docusaurus/Link';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import Layout from '@theme/Layout';
import Heading from '@theme/Heading';
import styles from './index.module.css';

function HomepageHeader() {
  const { siteConfig } = useDocusaurusContext();
  return (
    <header className={clsx('hero hero--primary', styles.heroBanner)}>
      <div className="container">
        <Heading as="h1" className="hero__title">
          {siteConfig.title}
        </Heading>
        <p className="hero__subtitle">{siteConfig.tagline}</p>
        <div className={styles.buttons}>
          <Link
            className="button button--secondary button--lg"
            to="/docs/intro">
            Get Started →
          </Link>
          <Link
            className="button button--outline button--lg"
            style={{ marginLeft: '1rem' }}
            href="https://github.com/fusioninfer/fusioninfer">
            GitHub
          </Link>
        </div>
      </div>
    </header>
  );
}

type FeatureItem = {
  title: string;
  description: JSX.Element;
  icon: string;
};

const FeatureList: FeatureItem[] = [
  {
    title: 'Unified Inference Platform',
    icon: '🔄',
    description: (
      <>
        Single CRD to manage inference workloads with automatic LeaderWorkerSet,
        PodGroup, InferencePool, and HTTPRoute generation.
      </>
    ),
  },
  {
    title: 'Intelligent Routing',
    icon: '🧠',
    description: (
      <>
        Built-in support for prefix-cache, KV-cache utilization, queue-size,
        LoRA affinity, and P/D disaggregation routing strategies.
      </>
    ),
  },
  {
    title: 'Gang Scheduling',
    icon: '⚡',
    description: (
      <>
        Automatic Volcano PodGroup integration ensures all pods for multi-node
        inference start together or not at all.
      </>
    ),
  },
  {
    title: 'Multi-Node Inference',
    icon: '🌐',
    description: (
      <>
        Native support for distributed inference with Ray integration and
        per-replica LeaderWorkerSet management.
      </>
    ),
  },
  {
    title: 'P/D Disaggregation',
    icon: '🔀',
    description: (
      <>
        First-class support for prefill/decode disaggregated architectures
        with automatic component separation and routing.
      </>
    ),
  },
  {
    title: 'Gateway API Native',
    icon: '🚀',
    description: (
      <>
        Seamless integration with Kubernetes Gateway API and Gateway API
        Inference Extension for production-grade traffic management.
      </>
    ),
  },
];

function Feature({ title, icon, description }: FeatureItem) {
  return (
    <div className={clsx('col col--4')}>
      <div className="text--center padding-horiz--md" style={{ marginBottom: '2rem' }}>
        <div style={{ fontSize: '3rem', marginBottom: '1rem' }}>{icon}</div>
        <Heading as="h3">{title}</Heading>
        <p>{description}</p>
      </div>
    </div>
  );
}

function HomepageFeatures() {
  return (
    <section className={styles.features}>
      <div className="container">
        <div className="row">
          {FeatureList.map((props, idx) => (
            <Feature key={idx} {...props} />
          ))}
        </div>
      </div>
    </section>
  );
}

function ArchitectureDiagram() {
  return (
    <section style={{ padding: '4rem 0', background: 'var(--ifm-background-surface-color)' }}>
      <div className="container">
        <Heading as="h2" className="text--center" style={{ marginBottom: '2rem' }}>
          Architecture Overview
        </Heading>
        <div className="text--center">
          <pre style={{
            textAlign: 'left',
            display: 'inline-block',
            padding: '2rem',
            borderRadius: '8px',
            background: 'var(--ifm-background-color)',
            fontSize: '0.85rem',
            lineHeight: '1.4',
          }}>
{`┌─────────────────────────────────────────────────────────────────────────────┐
│                            InferenceService CR                              │
│  roles:                                                                     │
│    - router (HTTPRoute, InferencePool, EPP)                                 │
│    - prefiller/decoder/worker (LeaderWorkerSet, PodGroup)                   │
└───────────────────────────────────┬─────────────────────────────────────────┘
                                    │
                    ┌───────────────┴───────────────┐
                    │  InferenceService Controller  │
                    └───────────────┬───────────────┘
                                    │
        ┌───────────────┬───────────┼───────────────┬───────────────┐
        ▼               ▼           ▼               ▼               ▼
┌───────────────┐ ┌───────────┐ ┌───────────┐ ┌───────────┐ ┌───────────────┐
│ HTTPRoute     │ │ Inference │ │    EPP    │ │  Leader   │ │   PodGroup    │
│ (Gateway API) │ │   Pool    │ │ Deployment│ │ WorkerSet │ │   (Volcano)   │
└───────────────┘ └───────────┘ └───────────┘ └───────────┘ └───────────────┘`}
          </pre>
        </div>
      </div>
    </section>
  );
}

export default function Home(): JSX.Element {
  const { siteConfig } = useDocusaurusContext();
  return (
    <Layout
      title={siteConfig.title}
      description="Unified Kubernetes-native LLM inference platform">
      <HomepageHeader />
      <main>
        <HomepageFeatures />
        <ArchitectureDiagram />
      </main>
    </Layout>
  );
}

