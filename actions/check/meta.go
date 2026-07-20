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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/siyuan-note/bazaar/actions/util"
	"github.com/siyuan-note/bazaar/rules"
)

const (
	checkMetaCommentStart = "<!-- bazaar-check-meta"
	checkMetaSchemaV      = 1
	checkMetaMaxAge       = 24 * time.Hour
)

// checkMetaBackoffIntervals 结果未变时的下次全检间隔（按 unchanged_streak 取档；超出用最后一档）。
var checkMetaBackoffIntervals = []time.Duration{
	20 * time.Minute,  // 0
	40 * time.Minute,  // 1
	60 * time.Minute,  // 2
	80 * time.Minute,  // 3
	100 * time.Minute, // 4
	2 * time.Hour,     // 5
	160 * time.Minute, // 6
	200 * time.Minute, // 7
	4 * time.Hour,     // 8
	5 * time.Hour,     // 9
	6 * time.Hour,     // 10
	8 * time.Hour,     // ≥11
}

// CheckMeta 写入 check-result 评论开头的调度元数据（JSON）。
type CheckMeta struct {
	V               int               `json:"v"`
	CheckedAt       string            `json:"checked_at"`
	ResultHash      string            `json:"result_hash"`
	UnchangedStreak int               `json:"unchanged_streak"`
	NextDueAt       string            `json:"next_due_at"`
	FP              *CheckFingerprint `json:"fp,omitempty"`
}

// CheckFingerprint 目标包仓 Latest Release 廉价指纹；纯下架 / 无目标仓时为 nil。
type CheckFingerprint struct {
	Repo         string `json:"repo"`
	ReleaseID    int64  `json:"release_id"`
	Tag          string `json:"tag"`
	ZipID        int64  `json:"zip_id"`
	ZipUpdatedAt string `json:"zip_updated_at,omitempty"`
}

// parseCheckMetaFromComment 从 PR 评论正文解析 bazaar-check-meta JSON。
func parseCheckMetaFromComment(body string) (*CheckMeta, bool) {
	i := strings.Index(body, checkMetaCommentStart)
	if i < 0 {
		return nil, false
	}
	rest := body[i+len(checkMetaCommentStart):]
	j := strings.Index(rest, "-->")
	if j < 0 {
		return nil, false
	}
	raw := strings.TrimSpace(rest[:j])
	if raw == "" {
		return nil, false
	}
	var m CheckMeta
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil, false
	}
	if m.V == 0 {
		m.V = checkMetaSchemaV
	}
	return &m, true
}

// marshalCheckMetaJSON 将 meta 序列化为紧凑 JSON（供模板嵌入 HTML 注释）。
func marshalCheckMetaJSON(m *CheckMeta) (string, error) {
	if m == nil {
		return "", fmt.Errorf("nil check meta")
	}
	b, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func backoffInterval(streak int) time.Duration {
	if streak < 0 {
		streak = 0
	}
	if streak >= len(checkMetaBackoffIntervals) {
		return checkMetaBackoffIntervals[len(checkMetaBackoffIntervals)-1]
	}
	return checkMetaBackoffIntervals[streak]
}

func fingerprintFromRelease(ownerRepo string, rel util.LatestRelease) *CheckFingerprint {
	if ownerRepo == "" {
		return nil
	}
	return &CheckFingerprint{
		Repo:         ownerRepo,
		ReleaseID:    rel.ID,
		Tag:          rel.Tag,
		ZipID:        rel.PackageZipAssetID,
		ZipUpdatedAt: rel.PackageZipUpdatedAt,
	}
}

func fingerprintFromCheckResult(r *CheckResult) *CheckFingerprint {
	if r == nil {
		return nil
	}
	for _, list := range [][]PackageCheck{r.Plugins, r.Themes, r.Icons, r.Templates, r.Widgets} {
		for _, pc := range list {
			if pc.RepoInfo.Path == "" {
				continue
			}
			return fingerprintFromRelease(pc.RepoInfo.Path, pc.Release)
		}
	}
	return nil
}

func fingerprintsEqual(a, b *CheckFingerprint) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Repo == b.Repo &&
		a.ReleaseID == b.ReleaseID &&
		a.Tag == b.Tag &&
		a.ZipID == b.ZipID &&
		a.ZipUpdatedAt == b.ZipUpdatedAt
}

// computeResultHash 对检查结果做稳定摘要（不含时间戳），用于判断「结果是否变化」。
func computeResultHash(r *CheckResult) string {
	if r == nil {
		return ""
	}
	type pkgHash struct {
		Path              string   `json:"path"`
		MaintainerChanged bool     `json:"maintainer_changed,omitempty"`
		ReleaseID         int64    `json:"release_id,omitempty"`
		Tag               string   `json:"tag,omitempty"`
		ZipID             int64    `json:"zip_id,omitempty"`
		Issues            []string `json:"issues,omitempty"`
	}
	payload := struct {
		ParseError string    `json:"parse_error,omitempty"`
		FlowError  string    `json:"flow_error,omitempty"`
		Packages   []pkgHash `json:"packages,omitempty"`
		Deleted    []string  `json:"deleted,omitempty"`
	}{
		ParseError: r.ParseError,
		FlowError:  r.FlowError,
	}
	appendPkgs := func(typ rules.PackageType, list []PackageCheck) {
		for _, pc := range list {
			issues := make([]string, 0, len(pc.Issues)*2)
			for _, iss := range pc.Issues {
				issues = append(issues, iss.MessageZh, iss.MessageEn)
			}
			payload.Packages = append(payload.Packages, pkgHash{
				Path:              typ.String() + ":" + pc.RepoInfo.Path,
				MaintainerChanged: pc.MaintainerChanged,
				ReleaseID:         pc.Release.ID,
				Tag:               pc.Release.Tag,
				ZipID:             pc.Release.PackageZipAssetID,
				Issues:            issues,
			})
		}
	}
	appendPkgs(rules.TypePlugin, r.Plugins)
	appendPkgs(rules.TypeTheme, r.Themes)
	appendPkgs(rules.TypeIcon, r.Icons)
	appendPkgs(rules.TypeTemplate, r.Templates)
	appendPkgs(rules.TypeWidget, r.Widgets)

	appendDel := func(typ rules.PackageType, paths []string) {
		for _, p := range paths {
			payload.Deleted = append(payload.Deleted, typ.String()+":"+p)
		}
	}
	appendDel(rules.TypePlugin, r.PluginsDeleted)
	appendDel(rules.TypeTheme, r.ThemesDeleted)
	appendDel(rules.TypeIcon, r.IconsDeleted)
	appendDel(rules.TypeTemplate, r.TemplatesDeleted)
	appendDel(rules.TypeWidget, r.WidgetsDeleted)

	b, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:8])
}

// buildNextCheckMeta 根据上次 meta 与本轮结果生成新的调度元数据。
func buildNextCheckMeta(prev *CheckMeta, result *CheckResult, now time.Time) *CheckMeta {
	if now.IsZero() {
		now = time.Now()
	}
	now = now.UTC()
	hash := computeResultHash(result)
	streak := 0
	if prev != nil && prev.ResultHash != "" && prev.ResultHash == hash {
		streak = prev.UnchangedStreak + 1
	}
	return &CheckMeta{
		V:               checkMetaSchemaV,
		CheckedAt:       now.Format(time.RFC3339),
		ResultHash:      hash,
		UnchangedStreak: streak,
		NextDueAt:       now.Add(backoffInterval(streak)).Format(time.RFC3339),
		FP:              fingerprintFromCheckResult(result),
	}
}

// shouldScheduleRecheck 判断定时任务是否应对该 PR 做完整检查。
// force 时始终复检；探测失败时 fail-open（复检）。
func shouldScheduleRecheck(meta *CheckMeta, currentFP *CheckFingerprint, probeErr error, now time.Time, force bool) (reason string, ok bool) {
	if force {
		return "force", true
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	if meta == nil {
		return "no-meta", true
	}
	if probeErr != nil {
		return "probe-error", true
	}
	if currentFP != nil && !fingerprintsEqual(currentFP, meta.FP) {
		return "fp-changed", true
	}
	if due, err := time.Parse(time.RFC3339, meta.NextDueAt); err == nil {
		if !now.Before(due) {
			return "backoff-due", true
		}
	} else if meta.NextDueAt == "" {
		return "no-next-due", true
	}
	if checked, err := time.Parse(time.RFC3339, meta.CheckedAt); err == nil {
		if now.Sub(checked) >= checkMetaMaxAge {
			return "max-age", true
		}
	}
	return "skip", false
}
