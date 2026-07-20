# SiYuan community bazaar <a title="Hits" target="_blank" href="https://github.com/siyuan-note/bazaar"><img src="https://hits.b3log.org/siyuan-note/bazaar.svg"></a>

**English** | [简体中文](./README.zh-CN.md)

## Bazaar package development samples

- [SiYuan plugin sample](https://github.com/siyuan-note/plugin-sample), maintained by the SiYuan team; please refer to the README in the repository for plugin-related specifications.
- [SiYuan plugin sample (Vite & Svelte)](https://github.com/siyuan-note/plugin-sample-vite-svelte), maintained by the community
- [SiYuan plugin sample (Vite & Vue3)](https://github.com/siyuan-note/plugin-sample-vite-vue), maintained by the community
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

   - One `owner/repo` per line, with no extra commas or empty lines.
   - Example: `siyuan-note/plugin-sample`.
   - Each PR may only be one of: add exactly 1 new package; change maintainer (add 1 new `owner/repo` and delete the old `owner/repo` with the same type and same GitHub repository name); or delist one or more packages only. Do not mix adding/changing with unrelated delistings in the same PR.

3. **Open a PR**

   Commit your changes and open a Pull Request to the `main` branch of this repo.

4. **Wait for review and merge**

   The PR Check workflow runs automatically to verify that the new package meets bazaar rules (e.g. release, required files, metadata fields). Maintainers will also review. Please make changes as requested.

   If the PR Check workflow fails, update your changes using the check output. Do not open a new pull request. After you fix the package repo (e.g. update the Latest Release / `package.zip`), a scheduled job re-checks open PRs with the `ci-failed` label about every two hours and updates the check comment. Maintainers can also add the `Check` label or run the workflow manually for an immediate re-check.

5. **Successfully listed**

   After the review passes and the PR is merged, the bazaar index will update within minutes and the package will appear in the SiYuan bazaar (you may need to restart SiYuan once to refresh the bazaar index cache).

## Updating a bazaar package

No need to open another PR. Release a new version in your package repository. The bazaar index will pull updates automatically.

Under normal circumstances, the community bazaar repo updates the index and deploys every one to three hours. You can check the deployment status on the [Stage workflow page](https://github.com/siyuan-note/bazaar/actions/workflows/stage.yml).

If it has not been updated for a long time, there may be an issue with the update (for example, the metadata version was not bumped). First check whether your repository has an open [Stage check failure issue](https://github.com/siyuan-note/bazaar/issues?q=is%3Aissue+is%3Aopen+label%3Astage-fail) (label `stage-fail`); you can also inspect the latest Stage workflow logs.

## Changing maintainers

If the original author can no longer maintain a listed bazaar package, a new maintainer may take it over. Changing maintainers requires a dedicated PR (one package per PR) and will only be merged after the original maintainer confirms.

### Submission process

1. **Prepare the new repository**

   The new maintainer should have a GitHub repository that can publish Releases (common approaches: fork the original repository, or have the original author transfer the repository). The repository must contain a valid bazaar package and publish a Latest Release that includes `package.zip`.

2. **Edit the bazaar package list TXT file**

   In the list file for the corresponding type: remove the old `owner/repo` line and add the new `owner/repo` line.

   - Example: change `alice/foo-plugin` to `bob/foo-plugin`.
   - The `name` in the package metadata should stay the same as the previously listed package so users still recognize it as the same bazaar package; the `url` must be updated to the new repository address.
   - A maintainer change counts as a package update: the manifest `version` must be higher than the previously listed version, and you must publish a Latest Release with a new `package.zip`.

3. **Open a PR and ask the original maintainer to confirm**

   After opening a Pull Request to the `main` branch of this repository, `@` the original maintainer in the PR, explain why you are taking over and your maintenance plan, and ask them to confirm they agree to the change.

   If the original maintainer does not agree, the PR will not be merged.

4. **Wait for review and merge**

   PR Check will verify the new repository against the listing rules. After the original maintainer confirms and checks pass, maintainers will merge the PR. Once merged, the bazaar index will update to the new `owner/repo`.

### About download counts and other statistics

Bazaar package statistics are currently keyed by `owner/repo`. After a maintainer change, statistics from the old repository (such as download counts) are **not inherited by default**.

To migrate statistics, all of the following are required:

1. The new maintainer requests the migration in the PR, or by opening a new [issue](https://github.com/siyuan-note/bazaar/issues) in this repository;
2. The original author replies and agrees;
3. After confirmation, a SiYuan maintainer migrates the data manually on the server side.

## Why is the repo named bazaar?

The name is inspired by the book _[The Cathedral and the Bazaar](https://en.wikipedia.org/wiki/The_Cathedral_and_the_Bazaar)_. The goal is not to be unconventional, but to continue the tradition of open source software.

## Other questions

Please open an [issue](https://github.com/siyuan-note/bazaar/issues).
