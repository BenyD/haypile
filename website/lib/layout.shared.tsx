import type { BaseLayoutProps } from 'fumadocs-ui/layouts/shared';
import { HaypileMark } from '@/components/logo';
import { appName, gitConfig } from './shared';

export function baseOptions(): BaseLayoutProps {
  return {
    nav: {
      title: (
        <>
          <HaypileMark className="size-5" />
          {appName}
        </>
      ),
    },
    links: [{ text: 'Docs', url: '/docs' }],
    githubUrl: `https://github.com/${gitConfig.user}/${gitConfig.repo}`,
  };
}
