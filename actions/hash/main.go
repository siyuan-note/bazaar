// SiYuan community bazaar.
// Copyright (c) 2021-present, b3log.org
//
// Bazaar is licensed under Mulan PSL v2.
// You can use this software according to the terms and conditions of the Mulan PSL v2.
// You may obtain a copy of Mulan PSL v2 at:
//         http://license.coscl.org.cn/MulanPSL2
// THIS SOFTWARE IS PROVIDED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OF ANY KIND, EITHER EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT, MERCHANTABILITY OR FIT FOR A PARTICULAR PURPOSE.
// See the Mulan PSL v2 for more details.

package hash

import (
	"os"
	"os/exec"
	"strings"

	"github.com/88250/gulu"
)

var logger = gulu.Log.NewLogger(os.Stdout)

func main() {
	logger.Infof("bazaar is hashing...")

	cmd := exec.Command("git", "rev-parse", "HEAD")
	data, err := cmd.CombinedOutput()
	if nil != err {
		logger.Fatalf("get git hash failed: %s", err)
	}
	hash := strings.TrimSpace(string(data))
	logger.Infof("bazaar [%s]", hash)

	stageIndex(hash, "themes")
	stageIndex(hash, "templates")
	stageIndex(hash, "icons")
	stageIndex(hash, "widgets")
	stageIndex(hash, "plugins")

	logger.Infof("indexed bazaar")
}
