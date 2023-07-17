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
	Icons: []Icon{
		{
			RepoInfo: RepoInfo{
				Path: "siyuan-note/icon-sample",
				Home: "https://github.com/siyuan-note/icon-sample",
			},
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
				IconJson: File{
					Pass: true,
					URL:  "https://github.com/siyuan-note/icon-sample/blob/95e07499bd1e0880155134628aacc4d07da419aa/icon.json",
				},
				IconPng: File{
					Pass: true,
					URL:  "https://github.com/siyuan-note/icon-sample/blob/95e07499bd1e0880155134628aacc4d07da419aa/icon.png",
				},
				PreviewPng: File{
					Pass: true,
					URL:  "https://github.com/siyuan-note/icon-sample/blob/95e07499bd1e0880155134628aacc4d07da419aa/preview.png",
				},
				ReadmeMd: File{
					Pass: true,
					URL:  "https://github.com/siyuan-note/icon-sample/blob/95e07499bd1e0880155134628aacc4d07da419aa/README.md",
				},
			},
			Attrs: Attrs{
				Pass: true,
				Name: Name{
					Pass:   true,
					Unique: true,
					Valid:  true,
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
			RepoInfo: RepoInfo{
				Path: "siyuan-note/icon-sample",
				Home: "https://github.com/siyuan-note/icon-sample",
			},
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
				IconJson: File{
					Pass: false,
				},
				IconPng: File{
					Pass: false,
				},
				PreviewPng: File{
					Pass: false,
				},
				ReadmeMd: File{
					Pass: false,
				},
			},
			Attrs: Attrs{
				Pass: false,
				Name: Name{
					Pass:   false,
					Unique: false,
					Valid:  false,
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
	Plugins: []Plugin{
		{
			RepoInfo: RepoInfo{
				Path: "siyuan-note/plugin-sample",
				Home: "https://github.com/siyuan-note/plugin-sample",
			},
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
					URL:  "https://github.com/siyuan-note/plugin-sample/blob/979f77bbeec0bc9d123305a7e18d1936ae67b009/icon.png",
				},
				PluginJson: File{
					Pass: true,
					URL:  "https://github.com/siyuan-note/plugin-sample/blob/979f77bbeec0bc9d123305a7e18d1936ae67b009/plugin.json",
				},
				PreviewPng: File{
					Pass: true,
					URL:  "https://github.com/siyuan-note/plugin-sample/blob/979f77bbeec0bc9d123305a7e18d1936ae67b009/preview.png",
				},
				ReadmeMd: File{
					Pass: true,
					URL:  "https://github.com/siyuan-note/plugin-sample/blob/979f77bbeec0bc9d123305a7e18d1936ae67b009/README.md",
				},
			},
			Attrs: Attrs{
				Pass: true,
				Name: Name{
					Pass:   true,
					Unique: true,
					Valid:  true,
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
			RepoInfo: RepoInfo{
				Path: "siyuan-note/plugin-sample",
				Home: "https://github.com/siyuan-note/plugin-sample",
			},
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
				},
				PluginJson: File{
					Pass: false,
				},
				PreviewPng: File{
					Pass: false,
				},
				ReadmeMd: File{
					Pass: false,
				},
			},
			Attrs: Attrs{
				Pass: false,
				Name: Name{
					Pass:   false,
					Unique: false,
					Valid:  false,
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
	Templates: []Template{
		{
			RepoInfo: RepoInfo{
				Path: "siyuan-note/template-sample",
				Home: "https://github.com/siyuan-note/template-sample",
			},
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
					URL:  "https://github.com/siyuan-note/template-sample/blob/280b81c2ca51c2fccb65662a56c02fc2fb050a9d/icon.png",
				},
				PreviewPng: File{
					Pass: true,
					URL:  "https://github.com/siyuan-note/template-sample/blob/280b81c2ca51c2fccb65662a56c02fc2fb050a9d/preview.png",
				},
				ReadmeMd: File{
					Pass: true,
					URL:  "https://github.com/siyuan-note/template-sample/blob/280b81c2ca51c2fccb65662a56c02fc2fb050a9d/README.md",
				},
				TemplateJson: File{
					Pass: true,
					URL:  "https://github.com/siyuan-note/template-sample/blob/280b81c2ca51c2fccb65662a56c02fc2fb050a9d/template.json",
				},
			},
			Attrs: Attrs{
				Pass: true,
				Name: Name{
					Pass:   true,
					Unique: true,
					Valid:  true,
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
			RepoInfo: RepoInfo{
				Path: "siyuan-note/template-sample",
				Home: "https://github.com/siyuan-note/template-sample",
			},
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
				},
				PreviewPng: File{
					Pass: false,
				},
				ReadmeMd: File{
					Pass: false,
				},
				TemplateJson: File{
					Pass: false,
				},
			},
			Attrs: Attrs{
				Pass: false,
				Name: Name{
					Pass:   false,
					Unique: false,
					Valid:  false,
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
	Themes: []Theme{
		{
			RepoInfo: RepoInfo{
				Path: "siyuan-note/theme-sample",
				Home: "https://github.com/siyuan-note/theme-sample",
			},
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
					URL:  "https://github.com/siyuan-note/theme-sample/blob/14665b04a381b8265ed27e5a4ad0156e7c0c05cc/icon.png",
				},
				PreviewPng: File{
					Pass: true,
					URL:  "https://github.com/siyuan-note/theme-sample/blob/14665b04a381b8265ed27e5a4ad0156e7c0c05cc/preview.png",
				},
				ReadmeMd: File{
					Pass: true,
					URL:  "https://github.com/siyuan-note/theme-sample/blob/14665b04a381b8265ed27e5a4ad0156e7c0c05cc/README.md",
				},
				ThemeJson: File{
					Pass: true,
					URL:  "https://github.com/siyuan-note/theme-sample/blob/14665b04a381b8265ed27e5a4ad0156e7c0c05cc/theme.json",
				},
			},
			Attrs: Attrs{
				Pass: true,
				Name: Name{
					Pass:   true,
					Unique: true,
					Valid:  true,
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
			RepoInfo: RepoInfo{
				Path: "siyuan-note/theme-sample",
				Home: "https://github.com/siyuan-note/theme-sample",
			},
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
				},
				PreviewPng: File{
					Pass: false,
				},
				ReadmeMd: File{
					Pass: false,
				},
				ThemeJson: File{
					Pass: false,
				},
			},
			Attrs: Attrs{
				Pass: false,
				Name: Name{
					Pass:   false,
					Unique: false,
					Valid:  false,
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
	Widgets: []Widget{
		{
			RepoInfo: RepoInfo{
				Path: "siyuan-note/widget-sample",
				Home: "https://github.com/siyuan-note/widget-sample",
			},
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
					URL:  "https://github.com/siyuan-note/widget-sample/blob/272314c056116dc32afbe61c85d541a509157948/icon.png",
				},
				PreviewPng: File{
					Pass: true,
					URL:  "https://github.com/siyuan-note/widget-sample/blob/272314c056116dc32afbe61c85d541a509157948/preview.png",
				},
				ReadmeMd: File{
					Pass: true,
					URL:  "https://github.com/siyuan-note/widget-sample/blob/272314c056116dc32afbe61c85d541a509157948/README.md",
				},
				WidgetJson: File{
					Pass: true,
					URL:  "https://github.com/siyuan-note/widget-sample/blob/272314c056116dc32afbe61c85d541a509157948/widget.json",
				},
			},
			Attrs: Attrs{
				Pass: true,
				Name: Name{
					Pass:   true,
					Unique: true,
					Valid:  true,
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
			RepoInfo: RepoInfo{
				Path: "siyuan-note/widget-sample",
				Home: "https://github.com/siyuan-note/widget-sample",
			},
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
				},
				PreviewPng: File{
					Pass: false,
				},
				ReadmeMd: File{
					Pass: false,
				},
				WidgetJson: File{
					Pass: false,
				},
			},
			Attrs: Attrs{
				Pass: false,
				Name: Name{
					Pass:   false,
					Unique: false,
					Valid:  false,
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
