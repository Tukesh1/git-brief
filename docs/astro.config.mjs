// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

// https://astro.build/config
export default defineConfig({
	site: 'https://tukesh.github.io',
	base: '/git-brief',
	integrations: [
		starlight({
			title: 'git brief',
			social: [
				{ icon: 'github', label: 'GitHub', href: 'https://github.com/Tukesh1/git-brief' },
				{ icon: 'external', label: 'Portfolio', href: 'https://tukesh.in/' },
			],

			sidebar: [
				{
					label: 'Start Here',
					items: [
						{ label: 'Introduction', slug: 'start-here/introduction' },
						{ label: 'Quick Start', slug: 'start-here/quick-start' },
						{ label: 'Installation', slug: 'start-here/installation' },
					],
				},
				{
					label: 'Concepts',
					items: [
						{ label: 'Collectors', slug: 'concepts/collectors' },
						{ label: 'AI Generation', slug: 'concepts/ai-generation' },
						{ label: 'Date Intelligence', slug: 'concepts/date-intelligence' },
					],
				},
				{
					label: 'Guides',
					items: [
						{ label: 'Setup Wizard', slug: 'guides/setup-wizard' },
						{ label: 'Configuration', slug: 'guides/configuration' },
						{ label: 'Edge Cases', slug: 'guides/edge-cases' },
					],
				},
				{
					label: 'Reference',
					items: [
						{ label: 'CLI Commands', slug: 'reference/cli' },
						{ label: 'AI Providers', slug: 'reference/providers' },
					],
				},
			],
		}),
	],
});
