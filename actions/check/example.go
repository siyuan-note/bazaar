// SiYuan community bazaar.
// Copyright (c) 2021-present, b3log.org
//
// Bazaar is licensed under Mulan PSL v2.
// You can use this software according to the terms and conditions of the Mulan PSL v2.
// You may obtain a copy of Mulan PSL v2 at:
//         http://license.coscl.org.cn/MulanPSL2
// THIS SOFTWARE IS PROVIDED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OF ANY KIND, EITHER EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT, MERCHANTABILITY OR FIT FOR A PARTICULAR PURPOSE.
// See the Mulan PSL v2 for more details.

package main

var CheckResultTestExample = CheckResult{
	Icons: []IconRepo{
		{
			Name: "icon-sample",
			Path: "siyuan-note/icon-sample",
			Home: "https://github.com/siyuan-note/icon-sample",
			Release: Release{
				Pass: true,
				LatestRelease: LatestRelease{
					Pass: true,
					URL:  "https://github.com/siyuan-note/icon-sample/releases/tag/v0.0.1",
					PackageZip: PackageZip{
						Pass: true,
						URL:  "https://github.com/siyuan-note/icon-sample/releases/download/v0.0.1/package.zip",
					},
				},
			},
			Files: IconFiles{
				Pass: true,
				IconJs: File{
					Pass: true,
					Name: "icon.js",
					URL:  "https://github.com/siyuan-note/icon-sample/blob/95e07499bd1e0880155134628aacc4d07da419aa/icon.js",
				},
				IconJson: File{
					Pass: true,
					Name: "icon.json",
					URL:  "https://github.com/siyuan-note/icon-sample/blob/95e07499bd1e0880155134628aacc4d07da419aa/icon.json",
				},
				IconPng: File{
					Pass: true,
					Name: "icon.png",
					URL:  "https://github.com/siyuan-note/icon-sample/blob/95e07499bd1e0880155134628aacc4d07da419aa/icon.png",
				},
				PreviewPng: File{
					Pass: true,
					Name: "preview.png",
					URL:  "https://github.com/siyuan-note/icon-sample/blob/95e07499bd1e0880155134628aacc4d07da419aa/preview.png",
				},
				ReadmeMd: File{
					Pass: true,
					Name: "README.md",
					URL:  "https://github.com/siyuan-note/icon-sample/blob/95e07499bd1e0880155134628aacc4d07da419aa/README.md",
				},
			},
			Attrs: Attrs{
				Pass: true,
				Name: Attr{
					Pass:   true,
					Unique: true,
					Value:  "icon-sample",
				},
				Version: Attr{
					Pass:  true,
					Value: "0.0.1",
				},
				Author: Attr{
					Pass:  true,
					Value: "Vanessa",
				},
				URL: Attr{
					Pass:  true,
					Value: "https://github.com/siyuan-note/icon-sample",
				},
			},
		},
		{
			Name: "icon-sample",
			Path: "siyuan-note/icon-sample",
			Home: "https://github.com/siyuan-note/icon-sample",
			Release: Release{
				Pass: false,
				LatestRelease: LatestRelease{
					Pass: false,
					PackageZip: PackageZip{
						Pass: false,
					},
				},
			},
			Files: IconFiles{
				Pass: false,
				IconJs: File{
					Pass: false,
					Name: "icon.js",
				},
				IconJson: File{
					Pass: false,
					Name: "icon.json",
				},
				IconPng: File{
					Pass: false,
					Name: "icon.png",
				},
				PreviewPng: File{
					Pass: false,
					Name: "preview.png",
				},
				ReadmeMd: File{
					Pass: false,
					Name: "README.md",
				},
			},
			Attrs: Attrs{
				Pass: false,
				Name: Attr{
					Pass:   false,
					Unique: false,
				},
				Version: Attr{
					Pass: false,
				},
				Author: Attr{
					Pass: false,
				},
				URL: Attr{
					Pass: false,
				},
			},
		},
	},
	Plugins: []PluginRepo{
		{
			Name: "plugin-sample",
			Path: "siyuan-note/plugin-sample",
			Home: "https://github.com/siyuan-note/plugin-sample",
			Release: Release{
				Pass: true,
				LatestRelease: LatestRelease{
					Pass: true,
					URL:  "https://github.com/siyuan-note/plugin-sample/releases/tag/v0.0.1",
					PackageZip: PackageZip{
						Pass: true,
						URL:  "https://github.com/siyuan-note/plugin-sample/releases/download/v0.0.1/package.zip",
					},
				},
			},
			Files: PluginFiles{
				Pass: true,
				IconPng: File{
					Pass: true,
					Name: "icon.png",
					URL:  "https://github.com/siyuan-note/plugin-sample/blob/979f77bbeec0bc9d123305a7e18d1936ae67b009/icon.png",
				},
				IndexJs: File{
					Pass: true,
					Name: "index.js",
					URL:  "https://github.com/siyuan-note/plugin-sample/blob/979f77bbeec0bc9d123305a7e18d1936ae67b009/index.js",
				},
				PluginJson: File{
					Pass: true,
					Name: "plugin.json",
					URL:  "https://github.com/siyuan-note/plugin-sample/blob/979f77bbeec0bc9d123305a7e18d1936ae67b009/plugin.json",
				},
				PreviewPng: File{
					Pass: true,
					Name: "preview.png",
					URL:  "https://github.com/siyuan-note/plugin-sample/blob/979f77bbeec0bc9d123305a7e18d1936ae67b009/preview.png",
				},
				ReadmeMd: File{
					Pass: true,
					Name: "README.md",
					URL:  "https://github.com/siyuan-note/plugin-sample/blob/979f77bbeec0bc9d123305a7e18d1936ae67b009/README.md",
				},
			},
			Attrs: Attrs{
				Pass: true,
				Name: Attr{
					Pass:   true,
					Unique: true,
					Value:  "plugin-sample",
				},
				Version: Attr{
					Pass:  true,
					Value: "0.0.1",
				},
				Author: Attr{
					Pass:  true,
					Value: "Vanessa",
				},
				URL: Attr{
					Pass:  true,
					Value: "https://github.com/siyuan-note/plugin-sample",
				},
			},
		},
		{
			Name: "plugin-sample",
			Path: "siyuan-note/plugin-sample",
			Home: "https://github.com/siyuan-note/plugin-sample",
			Release: Release{
				Pass: false,
				LatestRelease: LatestRelease{
					Pass: false,
					PackageZip: PackageZip{
						Pass: false,
					},
				},
			},
			Files: PluginFiles{
				Pass: false,
				IconPng: File{
					Pass: false,
					Name: "icon.png",
				},
				IndexJs: File{
					Pass: false,
					Name: "index.js",
				},
				PluginJson: File{
					Pass: false,
					Name: "plugin.json",
				},
				PreviewPng: File{
					Pass: false,
					Name: "preview.png",
				},
				ReadmeMd: File{
					Pass: false,
					Name: "README.md",
				},
			},
			Attrs: Attrs{
				Pass: false,
				Name: Attr{
					Pass:   false,
					Unique: false,
				},
				Version: Attr{
					Pass: false,
				},
				Author: Attr{
					Pass: false,
				},
				URL: Attr{
					Pass: false,
				},
			},
		},
	},
	Templates: []TemplateRepo{
		{
			Name: "template-sample",
			Path: "siyuan-note/template-sample",
			Home: "https://github.com/siyuan-note/template-sample",
			Release: Release{
				Pass: true,
				LatestRelease: LatestRelease{
					Pass: true,
					URL:  "https://github.com/siyuan-note/template-sample/releases/tag/v0.0.1",
					PackageZip: PackageZip{
						Pass: true,
						URL:  "https://github.com/siyuan-note/template-sample/releases/download/v0.0.1/package.zip",
					},
				},
			},
			Files: TemplateFiles{
				Pass: true,
				IconPng: File{
					Pass: true,
					Name: "icon.png",
					URL:  "https://github.com/siyuan-note/template-sample/blob/280b81c2ca51c2fccb65662a56c02fc2fb050a9d/icon.png",
				},
				PreviewPng: File{
					Pass: true,
					Name: "preview.png",
					URL:  "https://github.com/siyuan-note/template-sample/blob/280b81c2ca51c2fccb65662a56c02fc2fb050a9d/preview.png",
				},
				ReadmeMd: File{
					Pass: true,
					Name: "README.md",
					URL:  "https://github.com/siyuan-note/template-sample/blob/280b81c2ca51c2fccb65662a56c02fc2fb050a9d/README.md",
				},
				TemplateJson: File{
					Pass: true,
					Name: "template.json",
					URL:  "https://github.com/siyuan-note/template-sample/blob/280b81c2ca51c2fccb65662a56c02fc2fb050a9d/template.json",
				},
			},
			Attrs: Attrs{
				Pass: true,
				Name: Attr{
					Pass:   true,
					Unique: true,
					Value:  "template-sample",
				},
				Version: Attr{
					Pass:  true,
					Value: "0.0.1",
				},
				Author: Attr{
					Pass:  true,
					Value: "Vanessa",
				},
				URL: Attr{
					Pass:  true,
					Value: "https://github.com/siyuan-note/template-sample",
				},
			},
		},
		{
			Name: "template-sample",
			Path: "siyuan-note/template-sample",
			Home: "https://github.com/siyuan-note/template-sample",
			Release: Release{
				Pass: false,
				LatestRelease: LatestRelease{
					Pass: false,
					PackageZip: PackageZip{
						Pass: false,
					},
				},
			},
			Files: TemplateFiles{
				Pass: false,
				IconPng: File{
					Pass: false,
					Name: "icon.png",
				},
				PreviewPng: File{
					Pass: false,
					Name: "preview.png",
				},
				ReadmeMd: File{
					Pass: false,
					Name: "README.md",
				},
				TemplateJson: File{
					Pass: false,
					Name: "template.json",
				},
			},
			Attrs: Attrs{
				Pass: false,
				Name: Attr{
					Pass:   false,
					Unique: false,
				},
				Version: Attr{
					Pass: false,
				},
				Author: Attr{
					Pass: false,
				},
				URL: Attr{
					Pass: false,
				},
			},
		},
	},
	Themes: []ThemeRepo{
		{
			Name: "theme-sample",
			Path: "siyuan-note/theme-sample",
			Home: "https://github.com/siyuan-note/theme-sample",
			Release: Release{
				Pass: true,
				LatestRelease: LatestRelease{
					Pass: true,
					URL:  "https://github.com/siyuan-note/theme-sample/releases/tag/v0.0.1",
					PackageZip: PackageZip{
						Pass: true,
						URL:  "https://github.com/siyuan-note/theme-sample/releases/download/v0.0.1/package.zip",
					},
				},
			},
			Files: ThemeFiles{
				Pass: true,
				IconPng: File{
					Pass: true,
					Name: "icon.png",
					URL:  "https://github.com/siyuan-note/theme-sample/blob/14665b04a381b8265ed27e5a4ad0156e7c0c05cc/icon.png",
				},
				PreviewPng: File{
					Pass: true,
					Name: "preview.png",
					URL:  "https://github.com/siyuan-note/theme-sample/blob/14665b04a381b8265ed27e5a4ad0156e7c0c05cc/preview.png",
				},
				ReadmeMd: File{
					Pass: true,
					Name: "README.md",
					URL:  "https://github.com/siyuan-note/theme-sample/blob/14665b04a381b8265ed27e5a4ad0156e7c0c05cc/README.md",
				},
				ThemeCss: File{
					Pass: true,
					Name: "theme.css",
					URL:  "https://github.com/siyuan-note/theme-sample/blob/14665b04a381b8265ed27e5a4ad0156e7c0c05cc/theme.css",
				},
				ThemeJson: File{
					Pass: true,
					Name: "theme.json",
					URL:  "https://github.com/siyuan-note/theme-sample/blob/14665b04a381b8265ed27e5a4ad0156e7c0c05cc/theme.json",
				},
			},
			Attrs: Attrs{
				Pass: true,
				Name: Attr{
					Pass:   true,
					Unique: true,
					Value:  "theme-sample",
				},
				Version: Attr{
					Pass:  true,
					Value: "0.0.1",
				},
				Author: Attr{
					Pass:  true,
					Value: "Vanessa",
				},
				URL: Attr{
					Pass:  true,
					Value: "https://github.com/siyuan-note/theme-sample",
				},
			},
		},
		{
			Name: "theme-sample",
			Path: "siyuan-note/theme-sample",
			Home: "https://github.com/siyuan-note/theme-sample",
			Release: Release{
				Pass: false,
				LatestRelease: LatestRelease{
					Pass: false,
					PackageZip: PackageZip{
						Pass: false,
					},
				},
			},
			Files: ThemeFiles{
				Pass: false,
				IconPng: File{
					Pass: false,
					Name: "icon.png",
				},
				PreviewPng: File{
					Pass: false,
					Name: "preview.png",
				},
				ReadmeMd: File{
					Pass: false,
					Name: "README.md",
				},
				ThemeCss: File{
					Pass: false,
					Name: "theme.css",
				},
				ThemeJson: File{
					Pass: false,
					Name: "theme.json",
				},
			},
			Attrs: Attrs{
				Pass: false,
				Name: Attr{
					Pass:   false,
					Unique: false,
				},
				Version: Attr{
					Pass: false,
				},
				Author: Attr{
					Pass: false,
				},
				URL: Attr{
					Pass: false,
				},
			},
		},
	},
	Widgets: []WidgetRepo{
		{
			Name: "widget-sample",
			Path: "siyuan-note/widget-sample",
			Home: "https://github.com/siyuan-note/widget-sample",
			Release: Release{
				Pass: true,
				LatestRelease: LatestRelease{
					Pass: true,
					URL:  "https://github.com/siyuan-note/widget-sample/releases/tag/v0.0.1",
					PackageZip: PackageZip{
						Pass: true,
						URL:  "https://github.com/siyuan-note/widget-sample/releases/download/v0.0.1/package.zip",
					},
				},
			},
			Files: WidgetFiles{
				Pass: true,
				IconPng: File{
					Pass: true,
					Name: "icon.png",
					URL:  "https://github.com/siyuan-note/widget-sample/blob/272314c056116dc32afbe61c85d541a509157948/icon.png",
				},
				IndexHtml: File{
					Pass: true,
					Name: "index.html",
					URL:  "https://github.com/siyuan-note/widget-sample/blob/272314c056116dc32afbe61c85d541a509157948/index.html",
				},
				PreviewPng: File{
					Pass: true,
					Name: "preview.png",
					URL:  "https://github.com/siyuan-note/widget-sample/blob/272314c056116dc32afbe61c85d541a509157948/preview.png",
				},
				ReadmeMd: File{
					Pass: true,
					Name: "README.md",
					URL:  "https://github.com/siyuan-note/widget-sample/blob/272314c056116dc32afbe61c85d541a509157948/README.md",
				},
				WidgetJson: File{
					Pass: true,
					Name: "widget.json",
					URL:  "https://github.com/siyuan-note/widget-sample/blob/272314c056116dc32afbe61c85d541a509157948/widget.json",
				},
			},
			Attrs: Attrs{
				Pass: true,
				Name: Attr{
					Pass:   true,
					Unique: true,
					Value:  "widget-sample",
				},
				Version: Attr{
					Pass:  true,
					Value: "0.0.1",
				},
				Author: Attr{
					Pass:  true,
					Value: "Vanessa",
				},
				URL: Attr{
					Pass:  true,
					Value: "https://github.com/siyuan-note/widget-sample",
				},
			},
		},
		{
			Name: "widget-sample",
			Path: "siyuan-note/widget-sample",
			Home: "https://github.com/siyuan-note/widget-sample",
			Release: Release{
				Pass: false,
				LatestRelease: LatestRelease{
					Pass: false,
					PackageZip: PackageZip{
						Pass: false,
					},
				},
			},
			Files: WidgetFiles{
				Pass: false,
				IconPng: File{
					Pass: false,
					Name: "icon.png",
				},
				IndexHtml: File{
					Pass: false,
					Name: "index.html",
				},
				PreviewPng: File{
					Pass: false,
					Name: "preview.png",
				},
				ReadmeMd: File{
					Pass: false,
					Name: "README.md",
				},
				WidgetJson: File{
					Pass: false,
					Name: "widget.json",
				},
			},
			Attrs: Attrs{
				Pass: false,
				Name: Attr{
					Pass:   false,
					Unique: false,
				},
				Version: Attr{
					Pass: false,
				},
				Author: Attr{
					Pass: false,
				},
				URL: Attr{
					Pass: false,
				},
			},
		},
	},
}
