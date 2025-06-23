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

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	// 文件/目录名称保留字
	RESERVED_WORDS = StringSet{
		"CON":  nil,
		"PRN":  nil,
		"AUX":  nil,
		"NUL":  nil,
		"COM0": nil,
		"COM1": nil,
		"COM2": nil,
		"COM3": nil,
		"COM4": nil,
		"COM5": nil,
		"COM6": nil,
		"COM7": nil,
		"COM8": nil,
		"COM9": nil,
		"LPT0": nil,
		"LPT1": nil,
		"LPT2": nil,
		"LPT3": nil,
		"LPT4": nil,
		"LPT5": nil,
		"LPT6": nil,
		"LPT7": nil,
		"LPT8": nil,
		"LPT9": nil,
	}
)

// isKeyInSet 判断字符串是否在集合中
func isKeyInSet(
	key string,
	set StringSet,
) (exist bool) {
	_, exist = set[key]

	return
}

// isValidName 判断资源名称是否有效
func isValidName(name string) (valid bool) {
	var err error

	// 是否为空字符串
	if name == "" {
		logger.Warnf("name is empty")
		return
	}

	// 是否均为可打印的 ASCii 字符
	if valid, err = regexp.MatchString("^[\\x20-\\x7E]+$", name); err != nil {
		panic(err)
	} else if !valid {
		logger.Warnf("name <\033[7m%s\033[0m> contains characters other than printable ASCII characters", name)
		return
	}

	// 是否均为有效字符
	if valid, err = regexp.MatchString("^[^\\\\/:*?\"<>|. ][^\\\\/:*?\"<>|]*[^\\\\/:*?\"<>|. ]$", name); err != nil {
		panic(err)
	} else if !valid {
		logger.Warnf("name <\033[7m%s\033[0m> contains invalid characters", name)
		return
	}

	// 是否为保留字
	// REF https://learn.microsoft.com/zh-cn/windows/win32/fileio/naming-a-file#naming-conventions
	if valid = !isKeyInSet(strings.ToUpper(name), RESERVED_WORDS); !valid {
		logger.Warnf("name <\033[7m%s\033[0m> is a reserved word", name)
		return
	}

	return
}

// buildFileRawURL 构造文件原始访问地址
func buildFileRawURL(
	repoOwner string,
	repoName string,
	hash string,
	filePath string,
) string {
	return fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", repoOwner, repoName, hash, filePath)
}

// buildFilePreviewURL 构造文件预览地址
func buildFilePreviewURL(
	repoOwner string,
	repoName string,
	hash string,
	filePath string,
) string {
	return fmt.Sprintf("https://github.com/%s/%s/blob/%s/%s", repoOwner, repoName, hash, filePath)
}

// buildRepoHomeURL 构造仓库主页地址
func buildRepoHomeURL(
	repoOwner string,
	repoName string,
) string {
	return fmt.Sprintf("https://github.com/%s/%s", repoOwner, repoName)
}
