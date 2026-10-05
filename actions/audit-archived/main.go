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
audit-archived 是供维护者偶尔手工运行的巡检工具，不参与任何 CI 工作流。

用途：找出「仍在集市清单中、但仓库已被作者归档且尚未登记弃用」的包，并顺带列出仓库不存在、
被 GitHub 禁用、清单里 owner/repo 已过期（仓库改名），以及反向的「已登记弃用但仓库已不再归档」。

归档可以被作者取消，取消后作者可能已恢复维护，此时原弃用条目就陈旧了，应复核是否删除。
归档状态只是当下的一次快照，本工具不记录历史，也不能当作长期结论。

用法（在仓库根目录执行；进度与告警走 stderr，不污染报告）：

	go run ./actions/audit-archived -out report.md

以 -out 指定文件由本程序直接写出 UTF-8，避免 Windows 上 `> report.md` 被 PowerShell
按控制台代码页重编码而损毁中文；不指定 -out 时报告写 stdout。

令牌解析顺序：GITHUB_TOKEN → PAT → gh CLI 凭据（gh auth token）。

为什么必须用 GraphQL：REST 的 archived_at 已被 GitHub 废弃，go-github v89 中已无该字段，
只有 GraphQL 的 Repository.archivedAt 能给出「何时归档」。
*/

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/88250/gulu"
	"github.com/google/go-github/v89/github"
	"github.com/siyuan-note/bazaar/actions/util"
	"github.com/siyuan-note/bazaar/rules"
)

const (
	BAZAAR_ROOT_PATH = "." // bazaar 仓库根目录，须在仓库根目录执行
	batchSize        = 80  // 每个 GraphQL 请求查询的仓库数（每批约消耗同数量的配额点）

	requestTimeout = 60 * time.Second // 单个请求超时时间
)

// logger 输出到 stderr，保证 stdout 只有可直接重定向保存的报告正文。
var logger = gulu.Log.NewLogger(os.Stderr)

// listedRepo 表示清单中的一条 owner/repo。
type listedRepo struct {
	PackageType rules.PackageType
	OwnerRepo   string
	Deprecated  bool // 是否已在 deprecated.json 中登记
}

// repoState 是 listedRepo 加上 GitHub 侧的仓库状态。
type repoState struct {
	listedRepo
	Exists        bool
	NameWithOwner string // 仓库改名后与 OwnerRepo 不同
	IsArchived    bool
	ArchivedAt    string
	IsDisabled    bool
	PushedAt      string // 最后一次推送时间（UTC），用于判断取消归档后是否已恢复维护
}

// graphQLRepo 对应 GraphQL 查询中的一个 repository 节点；仓库不存在或无权查看时为 null。
type graphQLRepo struct {
	NameWithOwner string  `json:"nameWithOwner"`
	IsArchived    bool    `json:"isArchived"`
	ArchivedAt    *string `json:"archivedAt"`
	IsDisabled    bool    `json:"isDisabled"`
	PushedAt      *string `json:"pushedAt"`
}

func main() {
	outPath := flag.String("out", "", "报告输出文件路径；为空时写到 stdout")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	reposByType, err := util.LoadReposByPackageType(BAZAAR_ROOT_PATH)
	if err != nil {
		logger.Fatalf("parse repos list failed: %s", err)
	}
	registry, err := util.ReadDeprecatedRegistry(filepath.Join(BAZAAR_ROOT_PATH, util.DeprecatedRegistryRelPath))
	if err != nil {
		logger.Fatalf("parse deprecation registry failed: %s", err)
	}
	// 只告警不中止：过期的弃用元数据（如条目指向已下架的包）不阻碍本次巡检。
	for _, issue := range util.ValidateDeprecatedRegistry(registry, reposByType) {
		logger.Errorf("deprecation registry issue: %s", issue.MessageEn)
	}

	token := resolveToken()
	if token == "" {
		logger.Fatalf("no GitHub token: set GITHUB_TOKEN or PAT, or sign in with `gh auth login`")
	}
	client, err := util.NewGitHubClient(token, requestTimeout)
	if err != nil {
		logger.Fatalf("create github client failed: %s", err)
	}

	listed := collectListed(reposByType, registry)
	logger.Infof("listed %d packages in %d types", len(listed), len(rules.AllPackageTypes()))

	states, err := queryStates(ctx, client, listed)
	if err != nil {
		logger.Fatalf("query repository states failed: %s", err)
	}

	out, closeOut, err := openReportWriter(*outPath)
	if err != nil {
		logger.Fatalf("open report output failed: %s", err)
	}
	defer closeOut()

	writeReport(out, states)
	if *outPath != "" {
		logger.Infof("report written to %s", *outPath)
	}
}

// openReportWriter 返回报告写入目标：路径为空时写 stdout，否则写文件。
// 由本程序直接写文件，可避免 Windows 上 PowerShell 重定向按控制台代码页重编码而损毁中文。
func openReportWriter(path string) (io.Writer, func(), error) {
	if path == "" {
		return os.Stdout, func() {}, nil
	}
	file, err := os.Create(path)
	if err != nil {
		return nil, nil, err
	}
	return file, func() { _ = file.Close() }, nil
}

// resolveToken 依次尝试 GITHUB_TOKEN、PAT，最后回退到 gh CLI 凭据，便于本地手工运行。
func resolveToken() string {
	for _, key := range []string{"GITHUB_TOKEN", "PAT"} {
		if token := strings.TrimSpace(os.Getenv(key)); token != "" {
			return token
		}
	}
	out, err := exec.Command("gh", "auth", "token").Output()
	if err != nil {
		return ""
	}
	token := strings.TrimSpace(string(out))
	if token != "" {
		logger.Infof("using gh CLI credentials")
	}
	return token
}

// collectListed 汇总五个清单的全部条目，并标记哪些已登记弃用。
func collectListed(reposByType map[rules.PackageType][]string, registry *util.DeprecatedRegistry) []listedRepo {
	listed := make([]listedRepo, 0)
	for _, packageType := range rules.AllPackageTypes() {
		entries := registry.Entries(packageType)
		for _, ownerRepo := range reposByType[packageType] {
			_, deprecated := entries[ownerRepo]
			listed = append(listed, listedRepo{
				PackageType: packageType,
				OwnerRepo:   ownerRepo,
				Deprecated:  deprecated,
			})
		}
	}
	return listed
}

// queryStates 分批查询仓库状态，保持与入参相同的顺序。
func queryStates(ctx context.Context, client *github.Client, listed []listedRepo) ([]repoState, error) {
	states := make([]repoState, 0, len(listed))
	for start := 0; start < len(listed); start += batchSize {
		end := min(start+batchSize, len(listed))
		batch, err := queryBatch(ctx, client, listed[start:end])
		if err != nil {
			return nil, err
		}
		states = append(states, batch...)
		logger.Infof("queried %d/%d", end, len(listed))
	}
	return states, nil
}

// queryBatch 用 GraphQL 别名一次查询一批仓库；仓库不存在或无权查看时节点为 null。
func queryBatch(ctx context.Context, client *github.Client, chunk []listedRepo) ([]repoState, error) {
	var query strings.Builder
	query.WriteString("query {\n")
	for i, item := range chunk {
		owner, name, ok := strings.Cut(item.OwnerRepo, "/")
		if !ok {
			return nil, fmt.Errorf("invalid owner/repo [%s]", item.OwnerRepo)
		}
		fmt.Fprintf(&query,
			"  r%d: repository(owner: %s, name: %s) { nameWithOwner isArchived archivedAt isDisabled pushedAt }\n",
			i, strconv.Quote(owner), strconv.Quote(name))
	}
	query.WriteString("}")

	payload := map[string]any{"query": query.String()}
	req, err := client.NewRequest(ctx, "POST", "graphql", payload)
	if err != nil {
		return nil, err
	}
	var raw json.RawMessage
	if _, err := client.Do(req, &raw); err != nil {
		return nil, err
	}

	var resp struct {
		Data   map[string]*graphQLRepo `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, err
	}
	if len(resp.Errors) > 0 {
		messages := make([]string, 0, len(resp.Errors))
		for _, item := range resp.Errors {
			messages = append(messages, item.Message)
		}
		return nil, fmt.Errorf("graphql: %s", strings.Join(messages, "; "))
	}

	states := make([]repoState, 0, len(chunk))
	for i, item := range chunk {
		state := repoState{listedRepo: item}
		if node := resp.Data["r"+strconv.Itoa(i)]; node != nil {
			state.Exists = true
			state.NameWithOwner = node.NameWithOwner
			state.IsArchived = node.IsArchived
			state.IsDisabled = node.IsDisabled
			if node.ArchivedAt != nil {
				state.ArchivedAt = *node.ArchivedAt
			}
			if node.PushedAt != nil {
				state.PushedAt = *node.PushedAt
			}
		}
		states = append(states, state)
	}
	return states, nil
}

// writeReport 按类型与 owner/repo 稳定排序后输出 Markdown 报告，便于逐次 diff 结果。
func writeReport(w io.Writer, states []repoState) {
	slices.SortFunc(states, func(a, b repoState) int {
		if c := strings.Compare(a.PackageType.Plural(), b.PackageType.Plural()); c != 0 {
			return c
		}
		return strings.Compare(strings.ToLower(a.OwnerRepo), strings.ToLower(b.OwnerRepo))
	})

	var archivedNotDeprecated, archivedDeprecated, unarchivedDeprecated, disabledNotDeprecated, missing, renamed []repoState
	deprecatedCount := 0
	for _, state := range states {
		if state.Deprecated {
			deprecatedCount++
		}
		if !state.Exists {
			missing = append(missing, state)
			continue
		}
		if state.NameWithOwner != "" && !strings.EqualFold(state.NameWithOwner, state.OwnerRepo) {
			renamed = append(renamed, state)
		}
		switch {
		case state.IsArchived && !state.Deprecated:
			archivedNotDeprecated = append(archivedNotDeprecated, state)
		case state.IsArchived && state.Deprecated:
			archivedDeprecated = append(archivedDeprecated, state)
		case state.Deprecated:
			// 已登记弃用但仓库当前未归档：作者可能已取消归档并恢复维护，条目可能已陈旧。
			unarchivedDeprecated = append(unarchivedDeprecated, state)
		case state.IsDisabled:
			disabledNotDeprecated = append(disabledNotDeprecated, state)
		}
	}

	fmt.Fprintln(w, "# 集市包仓库状态巡检")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "- 生成时间（UTC）：%s\n", time.Now().UTC().Format("2006-01-02 15:04"))
	fmt.Fprintf(w, "- 清单条目：%d，其中已登记弃用：%d\n", len(states), deprecatedCount)
	fmt.Fprintf(w, "- 仓库可访问：%d\n", len(states)-len(missing))
	fmt.Fprintf(w, "- 已归档：%d（未登记弃用 %d / 已登记弃用 %d）\n",
		len(archivedNotDeprecated)+len(archivedDeprecated), len(archivedNotDeprecated), len(archivedDeprecated))
	fmt.Fprintf(w, "- 已登记弃用但仓库未归档：%d\n", len(unarchivedDeprecated))
	fmt.Fprintf(w, "- 已禁用且未登记弃用：%d\n", len(disabledNotDeprecated))
	fmt.Fprintf(w, "- 仓库不存在：%d\n", len(missing))
	fmt.Fprintf(w, "- 清单条目 owner/repo 已过期（仓库改名）：%d\n", len(renamed))
	fmt.Fprintln(w)

	writeArchivedSection(w, "已归档但未登记弃用", archivedNotDeprecated,
		"需要核实是否确已停止维护；确认后按 AGENTS.md「弃用集市包」流程提独立 PR，只改 `deprecated.json`。")
	writeArchivedSection(w, "已归档且已登记弃用", archivedDeprecated,
		"注册表与 GitHub 状态一致，无需处理。")
	writeUnarchivedSection(w, unarchivedDeprecated)
	writeArchivedSection(w, "已禁用但未登记弃用", disabledNotDeprecated,
		"仓库被 GitHub 禁用（多为违反服务条款），建议核实后决定是否弃用或下架。")

	fmt.Fprintf(w, "## 仓库不存在（%d）\n\n", len(missing))
	if len(missing) == 0 {
		fmt.Fprintln(w, "无。")
	} else {
		fmt.Fprintln(w, "GraphQL 把「不存在」与「无权查看」都返回为 `null`，请先确认是仓库已删除还是令牌看不到私有仓库；")
		fmt.Fprintln(w, "确属仓库已删除时，按 AGENTS.md「下架集市包」流程先开 issue 再直接提交到 `main`。")
		fmt.Fprintln(w)
		fmt.Fprintln(w, "| 类型 | 包 | 备注 |")
		fmt.Fprintln(w, "| --- | --- | --- |")
		for _, state := range missing {
			fmt.Fprintf(w, "| %s | `%s` | 仓库不存在或不可见 |\n", state.PackageType.Plural(), state.OwnerRepo)
		}
	}
	fmt.Fprintln(w)

	fmt.Fprintf(w, "## 清单条目 owner/repo 已过期（%d）\n\n", len(renamed))
	if len(renamed) == 0 {
		fmt.Fprintln(w, "无。")
	} else {
		fmt.Fprintln(w, "GitHub 会把旧名重定向到新名，PR Check 不受影响；如需修正清单行，请单独提 PR。")
		fmt.Fprintln(w)
		fmt.Fprintln(w, "| 类型 | 清单中的 owner/repo | 实际 nameWithOwner |")
		fmt.Fprintln(w, "| --- | --- | --- |")
		for _, state := range renamed {
			fmt.Fprintf(w, "| %s | `%s` | `%s` |\n", state.PackageType.Plural(), state.OwnerRepo, state.NameWithOwner)
		}
	}
}

// writeArchivedSection 输出一节归档/禁用条目表；标题中的数量按实际条目数给出。
func writeArchivedSection(w io.Writer, title string, states []repoState, note string) {
	fmt.Fprintf(w, "## %s（%d）\n\n", title, len(states))
	if len(states) == 0 {
		fmt.Fprintln(w, "无。")
		fmt.Fprintln(w)
		return
	}
	fmt.Fprintln(w, note)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "| 类型 / Type | 包 / Package | 归档时间（UTC） / Archived at | 备注 / Note |")
	fmt.Fprintln(w, "| --- | --- | --- | --- |")
	for _, state := range states {
		fmt.Fprintf(w, "| %s | `%s` | %s | %s |\n",
			state.PackageType.Plural(), state.OwnerRepo, formatTime(state.ArchivedAt), stateNote(state))
	}
	fmt.Fprintln(w)
}

// writeUnarchivedSection 输出「已登记弃用但仓库已不再归档」一节。
// 归档可被作者取消，取消后仓库可能已恢复维护，对应的弃用条目就该复核是否删除。
// 这一节包含因其它原因（如不兼容新版本）而弃用的包，属正常情况，需要人工看 `reason` 判定。
func writeUnarchivedSection(w io.Writer, states []repoState) {
	fmt.Fprintf(w, "## 已登记弃用但仓库未归档（%d）\n\n", len(states))
	if len(states) == 0 {
		fmt.Fprintln(w, "无。")
		fmt.Fprintln(w)
		return
	}
	fmt.Fprintln(w, "归档状态可被作者取消，本节列出所有「已登记弃用、但仓库当前未归档」的包。")
	fmt.Fprintln(w, "因其它原因（如不兼容新版本）而弃用的包出现在这里是正常的；需要处理的是 `reason` 声称")
	fmt.Fprintln(w, "「作者已归档仓库并停止维护」、而仓库现已恢复且「最后推送」较近的条目——")
	fmt.Fprintln(w, "应按 AGENTS.md「弃用集市包」流程提独立 PR 删除对应注册表条目。")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "| 类型 / Type | 包 / Package | 仓库状态 / Repo state | 最后推送（UTC） / Last push | 备注 / Note |")
	fmt.Fprintln(w, "| --- | --- | --- | --- | --- |")
	for _, state := range states {
		fmt.Fprintf(w, "| %s | `%s` | %s | %s | %s |\n",
			state.PackageType.Plural(), state.OwnerRepo, repoStateLabel(state), formatTime(state.PushedAt), stateNote(state))
	}
	fmt.Fprintln(w)
}

// repoStateLabel 描述「未归档」条目在 GitHub 上的当前状态。
func repoStateLabel(state repoState) string {
	if state.IsDisabled {
		return "已被禁用 / Disabled"
	}
	return "正常 / Active"
}

// stateNote 补一句备注：仓库改名时给出新名，被 GitHub 禁用时标出。
// 不能以「无归档时间」推断禁用——未归档的活跃仓库同样没有归档时间。
func stateNote(state repoState) string {
	notes := make([]string, 0, 2)
	if state.NameWithOwner != "" && !strings.EqualFold(state.NameWithOwner, state.OwnerRepo) {
		notes = append(notes, "已改名 → `"+state.NameWithOwner+"`")
	}
	if state.IsDisabled {
		notes = append(notes, "仓库已被 GitHub 禁用 / Disabled")
	}
	return strings.Join(notes, "；")
}

// formatTime 把 RFC3339 时间压缩成分钟精度，便于阅读与 diff。
func formatTime(raw string) string {
	if raw == "" {
		return "-"
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return raw
	}
	return parsed.UTC().Format("2006-01-02 15:04")
}
