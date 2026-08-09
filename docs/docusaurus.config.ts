import {themes as prismThemes} from 'prism-react-renderer';
import type {Config} from '@docusaurus/types';
import type * as Preset from '@docusaurus/preset-classic';

const config: Config = {
  title: 'ContainerWS',
  tagline: 'Linux workspace UI & API — Softwares, Docker, Kubernetes, Brew, MCP, and desktop',
  favicon: 'img/favicon.ico',

  future: {
    v4: true,
  },

  // Custom domain on GitHub Pages (DNS CNAME → izetmolla.github.io)
  url: 'https://containerws.izetmolla.com',
  baseUrl: '/',

  organizationName: 'izetmolla',
  projectName: 'containerws',

  trailingSlash: false,

  onBrokenLinks: 'throw',

  i18n: {
    defaultLocale: 'en',
    locales: ['en'],
  },

  presets: [
    [
      'classic',
      {
        docs: {
          sidebarPath: './sidebars.ts',
          editUrl: 'https://github.com/izetmolla/containerws/tree/main/docs/',
        },
        blog: false,
        theme: {
          customCss: './src/css/custom.css',
        },
      } satisfies Preset.Options,
    ],
  ],

  themeConfig: {
    image: 'img/social-card.png',
    colorMode: {
      respectPrefersColorScheme: true,
    },
    navbar: {
      title: 'ContainerWS',
      logo: {
        alt: 'ContainerWS',
        src: 'img/logo.svg',
      },
      items: [
        {
          type: 'docSidebar',
          sidebarId: 'docsSidebar',
          position: 'left',
          label: 'Docs',
        },
        {
          type: 'dropdown',
          label: 'Install',
          position: 'left',
          items: [
            {label: 'Native binary', to: '/docs/install/native-binary'},
            {label: 'Homebrew', to: '/docs/install/homebrew'},
            {label: 'Docker CLI', to: '/docs/install/docker-cli'},
            {label: 'Docker Compose', to: '/docs/install/docker-compose'},
            {label: 'Windows', to: '/docs/install/windows'},
            {label: 'Linux / WSL', to: '/docs/install/linux-wsl'},
            {label: 'Kubernetes', to: '/docs/install/kubernetes'},
            {label: 'Resources', to: '/docs/install/resources'},
          ],
        },
        {
          type: 'dropdown',
          label: 'Requirements',
          position: 'left',
          items: [
            {label: 'Host requirements', to: '/docs/getting-started/requirements'},
            {label: 'Security model', to: '/docs/reference/security'},
            {label: 'Environment variables', to: '/docs/configuration/environment'},
            {label: 'Ports & volumes', to: '/docs/configuration/ports-volumes'},
            {label: 'Resource allocation', to: '/docs/install/resources'},
          ],
        },
        {
          type: 'dropdown',
          label: 'Features',
          position: 'left',
          items: [
            {label: 'Softwares', to: '/docs/features/softwares'},
            {label: 'Brew', to: '/docs/features/brew'},
            {label: 'Analytics', to: '/docs/features/analytics'},
            {label: 'Docker', to: '/docs/features/docker'},
            {label: 'MCP', to: '/docs/features/mcp'},
            {label: 'CLI reference', to: '/docs/configuration/cli'},
          ],
        },
        {
          href: 'https://github.com/izetmolla/containerws',
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
            {label: 'Introduction', to: '/docs/intro'},
            {label: 'Getting started', to: '/docs/getting-started/overview'},
            {label: 'Install', to: '/docs/install/native-binary'},
            {label: 'CLI', to: '/docs/configuration/cli'},
          ],
        },
        {
          title: 'Features',
          items: [
            {label: 'Softwares', to: '/docs/features/softwares'},
            {label: 'Analytics', to: '/docs/features/analytics'},
            {label: 'MCP', to: '/docs/features/mcp'},
            {label: 'Docker', to: '/docs/features/docker'},
          ],
        },
        {
          title: 'More',
          items: [
            {
              label: 'GitHub',
              href: 'https://github.com/izetmolla/containerws',
            },
            {
              label: 'Homebrew tap',
              href: 'https://github.com/izetmolla/homebrew-tap',
            },
            {
              label: 'Author',
              href: 'mailto:izetmolla@icloud.com',
            },
          ],
        },
      ],
      copyright: `Copyright © ${new Date().getFullYear()} Izet Molla <izetmolla@icloud.com>. Built with Docusaurus.`,
    },
    prism: {
      theme: prismThemes.github,
      darkTheme: prismThemes.dracula,
      additionalLanguages: ['bash', 'yaml', 'json', 'go'],
    },
  } satisfies Preset.ThemeConfig,
};

export default config;
