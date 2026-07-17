// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

export default defineConfig({
	site: 'https://haypile.sh',
	integrations: [
		starlight({
			title: 'Haypile',
			description:
				'Private search and Q&A for your documents. One binary that watches your folders and answers questions about them, fully local and private.',
			social: [{ icon: 'github', label: 'GitHub', href: 'https://github.com/BenyD/haypile' }],
			editLink: {
				baseUrl: 'https://github.com/BenyD/haypile/edit/main/website/',
			},
			sidebar: [
				{ label: 'Getting started', slug: 'getting-started' },
				{
					label: 'Guides',
					items: [
						{ label: 'Search your documents', slug: 'guides/search' },
						{ label: 'Ask questions with a local LLM', slug: 'guides/ask' },
						{ label: 'Use Haypile from Claude Code', slug: 'guides/claude-code' },
						{ label: 'Configure a folder', slug: 'guides/folders' },
					],
				},
				{
					label: 'Reference',
					items: [
						{ label: 'CLI commands', slug: 'reference/cli' },
						{ label: 'REST API', slug: 'reference/api' },
						{ label: 'Configuration', slug: 'reference/configuration' },
					],
				},
				{
					label: 'Understand',
					items: [
						{ label: 'How search works', slug: 'explanation/how-it-works' },
						{ label: 'Privacy, verified', slug: 'explanation/privacy' },
					],
				},
			],
		}),
	],
});
