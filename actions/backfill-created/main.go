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

/*
backfill-created 是一次性工具：为 stage/*.json 的存量条目补上 created（首次进入集市的时间）。

状态：迁移已完成（2026-11，763 条存量条目全部回填，0 条未匹配）。此后 created 由 Stage 自身维护
（见 actions/stage/main.go 的 performStage），本工具不参与任何常规流程，也不被 CI 调用。

保留它的两个理由：
  - 下面这份「历史格式分期」是写工具时才摸清的，对任何要读 stage 文件历史的工具都仍然有用；
  - 它是这批 created 取值的唯一可复核手段：不带 -apply 直接跑一遍预览，即可确认现有数据与历史一致。

重跑注意：本工具取「史上最初一次出现」，而 Stage 运行时取「本次上架」——包下架后重新上架会重新
计时。因此重跑可能把重新上架过的包的 created 改早，与 Stage 今后写出的值不一致。预览出现差异时
先确认该包是否经历过下架再上架，不要盲目 -apply。

为什么必须从 git 历史推：stage 条目此前只有 updated（最近一次 Release 的发布时间），每次发版都被覆盖，
存量数据里没有任何「首次进入集市」的痕迹，只能回到提交历史里找该 owner/repo 第一次出现在
stage/<type>.json 的时间。

实现要点：
  - 每个 stage 文件只跑一次 `git log --reverse -p`，单趟扫描全部历史，边扫边记录每个
    owner/repo 首次出现（以新增的 `"url": "..."` 行为准）。
    逐个仓库跑 `git log -S` 会退化成 500 多次全历史 pickaxe，慢一到两个数量级。
  - 取各次出现中最小的 committer 时间，而不是「--reverse 后第一次遇到」：条目 updated 变化时会
    在按 updated 降序排列的文件里整体挪位，同一仓库的 url 行会被反复「删除再新增」。
  - **必须统一转成 UTC**：`%cI` 输出的是带提交者时区偏移的 RFC3339（如 2023-05-05T12:20:32+08:00），
    直接落盘会让 created 与 updated（恒为 UTC 的 Z 形式）混用两种时区表示。这里一律
    time.Time.UTC().Format(time.RFC3339) 后再写入。
  - 只写不改序：复用 util.MarshalSortedIndentedJSON（与 stage 写盘同一份实现），
    除新增 created 外文件逐字节不变。stage 包内有测试固定这一格式，改动它会让该测试失败。

历史格式分期（顶层 url 的形态，均已实测核对）：

	2021-08-26 起   `"url": "owner/repo@<40 位 git commit sha>"`，只有 url + updated
	2022-11-14 前后  `"url": "owner/repo"`（无 hash）—— 与新清单自己声明的裸 url 字面相同
	2023-05 起     增加 stars / openIssues / size
	2023-07 起     增加 installSize / package（package 内另有 `"url"` 键，是 https 链接或空串）
	2026-03-02     缩进修正提交（整文件重写，所有 url 行被删除再新增）
	2026-08-03 起  hash 改为 package.zip 内容的 sha256 前 7 位

所以解析只覆盖这些真实出现过的形态：缩进不敏感（4 格与 6 格都出现过）；带 scheme 或空串的 url
是 package.url，无条件忽略；无 hash 的裸 owner/repo 靠 hunk 上下文区分（见 classifyHunk）。

已知边界：
  - 仓库改名 / 转移后 owner/repo 变更，历史里只有新路径的记录，created 会从改名那次算起。
  - 在各自 stage 文件首个提交时就已存在的条目，只能退到该提交时间，早于此没有证据。
  - 包下架后重新上架，取的是「首次进入」而非最近这段连续上架史的起点。

用法（在仓库根目录执行；需完整历史，浅克隆会直接报错）：

	go run ./actions/backfill-created            # 预览，不写文件
	go run ./actions/backfill-created -apply     # 实际写入 stage/*.json
*/

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/siyuan-note/bazaar/actions/util"
	"github.com/siyuan-note/bazaar/rules"
)

const (
	BAZAAR_ROOT_PATH = "." // bazaar 仓库根目录，须在仓库根目录执行

	// commitMarker 是 git log --format 的自定义分隔符（SOH），避免与 diff 正文冲突。
	commitMarker = "\x01"
)

func main() {
	apply := flag.Bool("apply", false, "实际写入 stage/*.json；默认仅预览")
	flag.Parse()

	if err := ensureFullHistory(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}

	for _, packageType := range rules.AllPackageTypes() {
		if err := backfillType(packageType, *apply); err != nil {
			fmt.Fprintf(os.Stderr, "error: %s\n", err)
			os.Exit(1)
		}
	}

	if !*apply {
		fmt.Println("\ndry run: nothing written; re-run with -apply to update stage/*.json")
	}
}

// backfillType 为单一类型的 stage 文件回填 created。
func backfillType(packageType rules.PackageType, apply bool) error {
	stageRelPath := filepath.ToSlash(filepath.Join("stage", packageType.StageJSONFile()))
	stagePath := filepath.Join(BAZAAR_ROOT_PATH, "stage", packageType.StageJSONFile())

	firstSeen, err := firstSeenByRepo(stageRelPath)
	if err != nil {
		return fmt.Errorf("scan history of %s: %w", stageRelPath, err)
	}

	stageFile, err := util.ReadStageFile(stagePath)
	if err != nil {
		return fmt.Errorf("read %s: %w", stagePath, err)
	}

	var (
		changed, unchanged, unmatched int
	)
	for i := range stageFile.Repos {
		repo := &stageFile.Repos[i]
		ownerRepo, ok := util.OwnerRepoFromStageURL(repo.URL)
		if !ok {
			continue
		}

		derived, found := firstSeen[ownerRepo]
		if !found {
			// 历史完整时不会发生（列表里每条 url 都能在历史中找到）。出现了就只提示不改值，
			// 而不是猜一个时间盖掉已有 created。
			unmatched++
			fmt.Fprintf(os.Stderr, "warning: [%s] %s has no history match, keeping %q\n", packageType, ownerRepo, repo.Created)
			continue
		}

		next := earliestRFC3339(repo.Created, derived)
		if next == repo.Created {
			unchanged++
			continue
		}
		fmt.Printf("  [%s] %s: %q -> %q\n", packageType, ownerRepo, repo.Created, next)
		repo.Created = next
		changed++
	}

	fmt.Printf("[%s] %d entries: %d updated, %d unchanged, %d unmatched\n",
		packageType, len(stageFile.Repos), changed, unchanged, unmatched)

	if !apply || changed == 0 {
		return nil
	}
	data, err := util.MarshalSortedIndentedJSON(*stageFile)
	if err != nil {
		return fmt.Errorf("marshal %s: %w", stagePath, err)
	}
	if err = os.WriteFile(stagePath, data, 0644); err != nil {
		return fmt.Errorf("write %s: %w", stagePath, err)
	}
	fmt.Printf("[%s] wrote %s\n", packageType, stagePath)
	return nil
}

// firstSeenByRepo 单趟扫描 stage 文件的历史，返回每个 owner/repo 最早出现的 UTC 时间。
// 以新增行 `"url": "..."` 为「出现」凭据，形态判定见 classifyStageURL / classifyHunk。
func firstSeenByRepo(stageRelPath string) (map[string]time.Time, error) {
	cmd := exec.Command("git", "-C", BAZAAR_ROOT_PATH, "log", "--reverse",
		"--format="+commitMarker+"%cI", "--unified=0", "-p", "--", stageRelPath)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = os.Stderr
	if err = cmd.Start(); err != nil {
		return nil, err
	}

	firstSeen := make(map[string]time.Time)
	scanErr := scanHistory(stdout, func(ownerRepo string, at time.Time) {
		if prev, ok := firstSeen[ownerRepo]; !ok || at.Before(prev) {
			firstSeen[ownerRepo] = at
		}
	})
	if err = cmd.Wait(); err != nil {
		return nil, err
	}
	if scanErr != nil {
		return nil, scanErr
	}
	return firstSeen, nil
}

// scanHistory 解析 `git log --format=\x01%cI -p` 的输出流，把每个新增的 stage 条目 URL 交给 record。
// at 已归一化为 UTC。
//
// 按 hunk（@@ 分隔）缓冲新增行后再判定，而不是逐行独立判定：裸 owner/repo 需要与同 hunk 内的
// "updated" 行一起看，才能区分「新建条目」与「只改了 package.url」。
func scanHistory(r io.Reader, record func(ownerRepo string, at time.Time)) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)

	var (
		commitTime time.Time
		hunk       []string
	)
	flush := func() {
		defer func() { hunk = hunk[:0] }()
		if commitTime.IsZero() || len(hunk) == 0 {
			return
		}
		for _, ownerRepo := range classifyHunk(hunk) {
			record(ownerRepo, commitTime)
		}
	}

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, commitMarker) {
			flush()
			if t, err := time.Parse(time.RFC3339, strings.TrimPrefix(line, commitMarker)); err == nil {
				commitTime = t.UTC()
			}
			continue
		}
		if strings.HasPrefix(line, "@@") {
			flush()
			continue
		}
		// 只认 diff 的新增行；"+++ b/stage/xxx.json" 是文件头，不是内容。
		if !commitTime.IsZero() && strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			hunk = append(hunk, line[1:])
		}
	}
	flush()
	return scanner.Err()
}

// classifyHunk 从一个 hunk 的新增行中提取新增条目的 owner/repo。
//
// 历史格式分两段，必须分别处理：
//   - 大部分时期顶层 url 带 hash：`"url": "owner/repo@<git commit sha>"`（2021-08 .. 2026-07）
//     或 `"url": "owner/repo@<package.zip sha256 前 7 位>"`（2026-08 起）；两者都直接采信。
//   - 2022-11 前后短暂出现过不带 hash 的 `"url": "owner/repo"`。它与 package 内嵌的裸 url
//     （清单自己声明的 url 写成 owner/repo 而非 https 链接）字面完全一样，只能靠 hunk 上下文区分：
//     新建条目时该 hunk 一定同时新增了 "updated" 行，而只改 package.url 的 hunk 不会。
func classifyHunk(added []string) []string {
	hasEntryMarker := false
	for _, line := range added {
		if stageUpdatedLine.MatchString(line) {
			hasEntryMarker = true
			break
		}
	}

	var out []string
	for _, line := range added {
		m := stageURLLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		ownerRepo, hashed, ok := classifyStageURL(m[1])
		if !ok || (!hashed && !hasEntryMarker) {
			continue
		}
		out = append(out, ownerRepo)
	}
	return out
}

var (
	// stageURLLine 匹配 `"url": "..."` 键值行。缩进不敏感是必须的：2026-03-02 之前的 stage 文件
	// 缩进是错的，顶层 url 缩进 4 或 6 都出现过。
	stageURLLine = regexp.MustCompile(`^\s*"url"\s*:\s*"([^"]*)"`)
	// stageUpdatedLine 匹配 "updated" 键值行，用作「这是新建条目」的信号。
	stageUpdatedLine = regexp.MustCompile(`^\s*"updated"\s*:`)
)

// classifyStageURL 判断一个 url 值是否为 stage 条目顶层 url，返回 owner/repo 与是否带 hash。
// 历史里出现过的非条目 url 只有两种：带 scheme 的链接（package.url、funding）与空串。
func classifyStageURL(v string) (ownerRepo string, hashed bool, ok bool) {
	if v == "" || strings.Contains(v, "://") {
		return "", false, false
	}
	key, _, hashed := strings.Cut(v, "@")
	owner, repo, found := strings.Cut(key, "/")
	if !found || owner == "" || repo == "" {
		return "", false, false
	}
	return key, hashed, true
}

// earliestRFC3339 返回 existing 与 derived 中较早的一个（RFC3339 UTC）。
// 取较早值而非直接覆盖，是为了让本工具可重复执行。
func earliestRFC3339(existing string, derived time.Time) string {
	want := derived.UTC().Format(time.RFC3339)
	prev, err := time.Parse(time.RFC3339, existing)
	if err != nil {
		return want
	}
	if prev.Before(derived) {
		return prev.UTC().Format(time.RFC3339)
	}
	return want
}

// ensureFullHistory 拒绝浅克隆：历史被截断时回填结果会系统性偏晚，且不报错。
func ensureFullHistory() error {
	out, err := exec.Command("git", "-C", BAZAAR_ROOT_PATH, "rev-parse", "--is-shallow-repository").Output()
	if err != nil {
		return fmt.Errorf("git rev-parse --is-shallow-repository: %w", err)
	}
	if strings.TrimSpace(string(out)) == "true" {
		return fmt.Errorf("repository is a shallow clone; run `git fetch --unshallow` first")
	}
	return nil
}
