import type {ReactNode} from 'react';
import clsx from 'clsx';
import Link from '@docusaurus/Link';
import Heading from '@theme/Heading';
import styles from './styles.module.css';

type FeatureItem = {
  title: string;
  description: ReactNode;
  to: string;
};

const FeatureList: FeatureItem[] = [
  {
    title: 'Install anywhere',
    to: '/docs/install/native-binary',
    description: (
      <>
        Native binary, Homebrew, Docker CLI/Compose, Windows Desktop, Linux/WSL,
        or Kubernetes — pick the path that matches your host.
      </>
    ),
  },
  {
    title: 'Softwares & Brew',
    to: '/docs/features/softwares',
    description: (
      <>
        Catalog installs, install queue, package authoring, and Homebrew
        management with ownership switching.
      </>
    ),
  },
  {
    title: 'Docker, K8s & MCP',
    to: '/docs/features/mcp',
    description: (
      <>
        Nested Docker Engine UI, multi-cluster Kubernetes, and a full MCP tool
        surface for agents on <code>/mcp</code>.
      </>
    ),
  },
];

function Feature({title, description, to}: FeatureItem) {
  return (
    <div className={clsx('col col--4')}>
      <div className="text--center padding-horiz--md">
        <Heading as="h3">
          <Link to={to}>{title}</Link>
        </Heading>
        <p>{description}</p>
      </div>
    </div>
  );
}

export default function HomepageFeatures(): ReactNode {
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
