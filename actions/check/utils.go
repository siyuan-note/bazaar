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

import "fmt"

// isKeyInSet 判断字符串是否在集合中
func isKeyInSet(
	key string,
	set StringSet,
) (exist bool) {
	_, exist = set[key]

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
