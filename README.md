# SiYuan community bazaar <a title="Hits" target="_blank" href="https://github.com/siyuan-note/bazaar"><img src="https://hits.b3log.org/siyuan-note/bazaar.svg"></a>

English | [简体中文](./README_zh_CN.md)

## Bazaar package development samples

- [SiYuan plugin sample](https://github.com/siyuan-note/plugin-sample), please refer to the README in the repository for plugin-related specifications.
- [SiYuan plugin sample (Vite & Svelte)](https://github.com/siyuan-note/plugin-sample-vite-svelte)
- [SiYuan plugin sample (Vite & Vue3)](https://github.com/siyuan-note/plugin-sample-vite-vue)
- [SiYuan theme sample](https://github.com/siyuan-note/theme-sample)
- [SiYuan icon sample](https://github.com/siyuan-note/icon-sample)
- [SiYuan template sample](https://github.com/siyuan-note/template-sample)
- [SiYuan widget sample](https://github.com/siyuan-note/widget-sample)

## Submitting a bazaar package

If you have developed a plugin, theme, icon, template or widget and want to list it in the SiYuan community bazaar, follow these steps:

1. **Fork this repository**
   Fork [siyuan-note/bazaar](https://github.com/siyuan-note/bazaar) on GitHub. If you have already forked, sync with the latest main branch first.

2. **Edit the bazaar package list TXT file**
   In the repo root there are five list files: `plugins.txt`, `themes.txt`, `icons.txt`, `templates.txt`, `widgets.txt`.
   Add one line to the file that matches your package type. Format: `owner/repo` (owner is your GitHub username or org name, repo is the bazaar package repository name).

   - One `owner/repo` per line; no extra commas or empty lines.
   - Example: `siyuan-note/plugin-sample`.

3. **Open a PR**
   Commit your changes and open a Pull Request to the `main` branch of this repo.

4. **Wait for review and merge**
   CI will run to check that the new package meets bazaar rules (e.g. release, required files, manifest fields). Maintainers will also review; please make changes as requested.
   After the review passes and the PR is merged, the bazaar index will update within minutes and the package will appear in the SiYuan bazaar (you may need to restart SiYuan once to refresh the bazaar index cache).

## Updating a bazaar package

No need to open another PR. Release a new version in your package repository; the bazaar index will pull updates automatically.

Under normal circumstances, the community bazaar repo updates the index and deploys every hour. You can check the deployment status at [https://github.com/siyuan-note/bazaar/actions](https://github.com/siyuan-note/bazaar/actions).

## Why is the repo named bazaar?

The name is inspired by the book _[The Cathedral and the Bazaar](https://en.wikipedia.org/wiki/The_Cathedral_and_the_Bazaar)_. The goal is not to be unconventional, but to continue the tradition of open source software.

## Other questions

Please open an [issue](https://github.com/siyuan-note/bazaar/issues).
