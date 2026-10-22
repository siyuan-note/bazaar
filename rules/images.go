// SiYuan community bazaar.
// Copyright (c) 2021-present, b3log.org
//
// Bazaar is licensed under Mulan PSL v2.
// You can use this software according to the terms and conditions of the Mulan PSL v2.
// You may obtain a copy of Mulan PSL v2 at:
//         http://license.coscl.org.cn/MulanPSL2
// THIS SOFTWARE IS PROVIDED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OF ANY KIND, EITHER EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT, MERCHANTABILITY OR FIT FOR A PARTICULAR PURPOSE.
// See the Mulan PSL v2 for more details.

package rules

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	maxIconImageBytes    = 64 * 1024
	maxPreviewImageBytes = 512 * 1024
)

type packageImageSpec struct {
	field      string
	legacyName string
	label      string
	labelEn    string
	maxSize    int64
}

var packageImageSpecs = []packageImageSpec{
	{field: "icon", legacyName: "icon.png", label: "图标文件", labelEn: "icon file", maxSize: maxIconImageBytes},
	{field: "preview", legacyName: "preview.png", label: "预览图文件", labelEn: "preview image file", maxSize: maxPreviewImageBytes},
}

var imageMIMEByExtension = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".webp": "image/webp",
	".avif": "image/avif",
}

// normalizePackageImages 校验并归一化清单中的图标和预览图。
// 缺少字段时探测包根同名 PNG 文件；归一化后 nil 只保留给尚未迁移的旧 Stage 条目。
func normalizePackageImages(manifest map[string]any, pkg *Package, root string) []Issue {
	if pkg == nil {
		return nil
	}
	var issues []Issue
	for _, spec := range packageImageSpecs {
		fileName, imageIssues := normalizePackageImage(manifest, root, spec)
		issues = append(issues, imageIssues...)
		value := fileName
		switch spec.field {
		case "icon":
			pkg.Icon = &value
		case "preview":
			pkg.Preview = &value
		}
	}
	return issues
}

func normalizePackageImage(manifest map[string]any, root string, spec packageImageSpec) (string, []Issue) {
	raw, declared := manifest[spec.field]
	fileName := ""
	if declared {
		value, ok := raw.(string)
		if !ok {
			return "", []Issue{issue(
				fmt.Sprintf("清单字段 `%s` 若存在则必须是字符串文件名，例如 `%s`。", spec.field, spec.legacyName),
				fmt.Sprintf("If you include `%s`, it must be a string filename, e.g. `%s`.", spec.field, spec.legacyName),
			)}
		}
		fileName = value
		if fileName == "" || strings.TrimSpace(fileName) != fileName {
			return "", []Issue{issue(
				fmt.Sprintf("清单字段 `%s` 必须是无首尾空白的非空文件名；不需要图片时请删除该字段和包根的 `%s`。", spec.field, spec.legacyName),
				fmt.Sprintf("Manifest field `%s` must be a non-empty filename without leading or trailing whitespace. If no image is needed, remove the field and the `%s` file at the package root.", spec.field, spec.legacyName),
			)}
		}
		if filepath.IsAbs(fileName) || fileName == "." || fileName == ".." || strings.ContainsAny(fileName, `/\`) {
			return fileName, []Issue{issue(
				fmt.Sprintf("清单字段 `%s` 的值 `%s` 不是合法的包根文件名。图片必须直接放在 `package.zip` 包根，不能使用目录或绝对路径。", spec.field, fileName),
				fmt.Sprintf("Manifest field `%s` value `%s` isn't a valid package-root filename. Put the image directly at the `package.zip` root; directories and absolute paths aren't allowed.", spec.field, fileName),
			)}
		}
	} else if fileExistsCaseSensitive(root, spec.legacyName) {
		fileName = spec.legacyName
	} else {
		return "", nil
	}

	if !fileExistsCaseSensitive(root, fileName) {
		return fileName, []Issue{issue(
			fmt.Sprintf("清单字段 `%s` 声明了图片 `%s`，但 `package.zip` 包根中找不到该文件（文件名大小写必须一致）。", spec.field, fileName),
			fmt.Sprintf("Manifest field `%s` declares image `%s`, but that file isn't at the `package.zip` root (the filename is case-sensitive).", spec.field, fileName),
		)}
	}

	// 清单未声明该字段、仅校验包根同名文件时，消息不能写成「清单字段 icon 声明」，
	// 否则作者会被告知一个自己从未写过的声明（见 bazaar#2363）。
	zhSubject, enSubject := imageSubject(spec, fileName, declared)

	path := filepath.Join(root, fileName)
	info, err := os.Stat(path)
	if err != nil {
		return fileName, []Issue{issue(
			fmt.Sprintf("无法读取%s：%v。", zhSubject, err),
			fmt.Sprintf("Failed to inspect %s: %v.", enSubject, err),
		)}
	}
	if info.IsDir() {
		return fileName, []Issue{issue(
			fmt.Sprintf("%s是目录，必须改为普通图片文件。", zhSubject),
			fmt.Sprintf("%s is a directory; it must be a regular image file.", enSubject),
		)}
	}

	expectedMIME := ImageMIMEType(fileName)
	if expectedMIME == "" {
		return fileName, []Issue{issue(
			fmt.Sprintf("%s格式不受支持。仅支持 PNG、JPEG、WebP 和 AVIF，不支持 SVG。", zhSubject),
			fmt.Sprintf("%s has an unsupported format. Only PNG, JPEG, WebP, and AVIF are supported; SVG isn't supported.", enSubject),
		)}
	}
	if info.Size() > spec.maxSize {
		return fileName, []Issue{issue(
			fmt.Sprintf("%s大小为 %s，超过上限 %s。请压缩或缩小该文件后重新打包，并更新 GitHub Release 中的 `package.zip`。",
				zhSubject, formatByteSize(info.Size()), formatByteSize(spec.maxSize)),
			fmt.Sprintf("%s is %s, which exceeds the limit of %s. Please compress or shrink the file, repackage it, and update `package.zip` in the GitHub Release.",
				enSubject, formatByteSize(info.Size()), formatByteSize(spec.maxSize)),
		)}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fileName, []Issue{issue(
			fmt.Sprintf("无法读取%s：%v。", zhSubject, err),
			fmt.Sprintf("Failed to read %s: %v.", enSubject, err),
		)}
	}
	actualMIME := detectImageMIMEType(data)
	if actualMIME != expectedMIME {
		actual := "未知格式"
		actualEn := "an unknown format"
		if actualMIME != "" {
			actual = actualMIME
			actualEn = actualMIME
		}
		return fileName, []Issue{issue(
			fmt.Sprintf("%s扩展名对应 `%s`，但文件内容实际格式为 `%s`。请使用与实际格式一致的扩展名和内容。", zhSubject, expectedMIME, actual),
			fmt.Sprintf("%s uses extension type `%s`, but its contents are `%s`. Please make the extension and actual format match.", enSubject, expectedMIME, actualEn),
		)}
	}
	return fileName, nil
}

// imageSubject 返回报错中指向待校验图片的主语。
// 清单显式声明了字段时指出该声明；否则只说包根文件名，不强调字段与文件两种来源的差别，
// 两者都是受支持的使用方式。
func imageSubject(spec packageImageSpec, fileName string, declared bool) (zh, en string) {
	if declared {
		return fmt.Sprintf("清单字段 `%s` 声明的图片 `%s`", spec.field, fileName),
			fmt.Sprintf("Image `%s` declared by manifest field `%s`", fileName, spec.field)
	}
	return fmt.Sprintf("包根的%s `%s`", spec.label, fileName),
		fmt.Sprintf("%s `%s` at the package root", spec.labelEn, fileName)
}

// ImageMIMEType 根据支持的图片扩展名返回上传所需的 MIME；不支持时返回空字符串。
func ImageMIMEType(fileName string) string {
	return imageMIMEByExtension[strings.ToLower(filepath.Ext(fileName))]
}

func detectImageMIMEType(data []byte) string {
	if len(data) >= 8 && bytes.Equal(data[:8], []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}) {
		return "image/png"
	}
	if len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return "image/jpeg"
	}
	if len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		return "image/webp"
	}
	if isAVIF(data) {
		return "image/avif"
	}
	return ""
}

func isAVIF(data []byte) bool {
	if len(data) < 16 || string(data[4:8]) != "ftyp" {
		return false
	}
	boxSize := int(binary.BigEndian.Uint32(data[:4]))
	if boxSize == 0 {
		boxSize = len(data)
	}
	if boxSize < 16 || len(data) < boxSize {
		return false
	}
	if isAVIFBrand(data[8:12]) {
		return true
	}
	for offset := 16; offset+4 <= boxSize; offset += 4 {
		if isAVIFBrand(data[offset : offset+4]) {
			return true
		}
	}
	return false
}

func isAVIFBrand(brand []byte) bool {
	return string(brand) == "avif" || string(brand) == "avis"
}
