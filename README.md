# SiYuan community bazaar <a title="Hits" target="_blank" href="https://github.com/siyuan-note/bazaar"><img src="https://hits.b3log.org/siyuan-note/bazaar.svg"></a>

[中文](https://github.com/siyuan-note/bazaar/blob/main/README_zh_CN.md)

## Overview

The SiYuan community bazaar is divided into four parts:

* Theme bazaar
* Template bazaar
* Icon bazaar
* Widget bazaar

Please refer to the following methods for listing.

## Push to theme bazaar

Please make sure that the root path of your theme repository contains at least the following files before listing ([repo example](https://github.com/88250/Comfortably-Numb)):

* theme.css
* theme.json (please make sure the JSON format is correct)
* preview.png (please compress the image size within 512 KB)
* README.md (please note the case)
* Create a release on GitHub

After confirmation, please [create a pull request](https://docs.github.com/en/free-pro-team@latest/github/collaborating-with-issues-and-pull-requests/creating-a-pull-request) to the [Community Bazaar](https://github.com/siyuan-note/bazaar) repository and modify the themes.json file in it. This file is the index of all community theme repositories, the format is:

```json
{
   "repos": [
     "username/reponame"
   ]
}
```

If the theme you developed has an updated version, please remember:

* Update the version field in your theme.json
* Create a new release on GitHub

## Push to template bazaar

Please make sure that the root path of your template repository contains at least the following files before listing ([repo example](https://github.com/88250/November-Rain)):

* template.json (please make sure the JSON format is correct)
* preview.png (please compress the image size within 512 KB)
* README.md (please note the case)
* Create a release on GitHub

After confirmation, please [create a pull request](https://docs.github.com/en/free-pro-team@latest/github/collaborating-with-issues-and-pull-requests/creating-a-pull-request) to the [Community Bazaar](https://github.com/siyuan-note/bazaar) repository and modify the themes.json file in it. This file is the index file of all community template repositories, the format is:

```json
{
   "repos": [
     "username/reponame"
   ]
}
```

If the template you developed has an updated version, please remember:

* Update the version field in your template.json
* Create a new release on GitHub

## Push to icon bazaar

Please make sure that the root path of your icon repository contains at least the following files before listing:

* icon.js
* icon.json (please make sure the JSON format is correct)
* preview.png (please compress the image size within 512 KB)
* README.md (please note the case)
* Create a release on GitHub

After confirmation, please [create a pull request](https://docs.github.com/en/free-pro-team@latest/github/collaborating-with-issues-and-pull-requests/creating-a-pull-request) to the [Community Bazaar](https://github.com/siyuan-note/bazaar) repository and modify the consi.json file in it. This file is the index of all community icon repositories, the format is:

```json
{
   "repos": [
     "username/reponame"
   ]
}
```

If the icon you developed has an updated version, please remember:

* Update the version field in your icon.json
* Create a new release on GitHub

## Push to widget bazaar

Please make sure that the root path of your widget repository contains at least the following files before listing:

* index.html
* widget.json (please make sure the JSON format is correct)
* preview.png (please compress the image size within 512 KB)
* README.md (please note the case)
* Create a release on GitHub

After confirmation, please [create a pull request](https://docs.github.com/en/free-pro-team@latest/github/collaborating-with-issues-and-pull-requests/creating-a-pull-request) to the [Community Bazaar](https://github.com/siyuan-note/bazaar) repository and modify the consi.json file in it. This file is the index of all community icon repositories, the format is:

```json
{
   "repos": [
     "username/reponame"
   ]
}
```

If the widget you developed has an updated version, please remember:

* Update the version field in your widget.json
* Create a new release on GitHub

---

## Why is repo named bazaar?

The name of the repository is inspired by the book _[The Cathedral and the Bazaar](https://en.wikipedia.org/wiki/The_Cathedral_and_the_Bazaar)_. 

The original intention is not to be unconventional, but to continue the tradition of open source software.
