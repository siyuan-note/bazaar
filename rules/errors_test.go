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
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadManifestLocalizedErrors(t *testing.T) {
	t.Run("missing file", func(t *testing.T) {
		_, err := ReadManifest(filepath.Join(t.TempDir(), "plugin.json"))
		if err == nil {
			t.Fatal("expected error")
		}
		zh, en, ok := AsLocalized(err)
		if !ok {
			t.Fatalf("expected localized error, got %v", err)
		}
		if !strings.Contains(zh, "无法读取清单文件 plugin.json") {
			t.Fatalf("unexpected zh: %s", zh)
		}
		if !strings.Contains(en, "Cannot read manifest plugin.json") {
			t.Fatalf("unexpected en: %s", en)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "plugin.json")
		if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := ReadManifest(path)
		if err == nil {
			t.Fatal("expected error")
		}
		zh, en, ok := AsLocalized(err)
		if !ok {
			t.Fatalf("expected localized error, got %v", err)
		}
		if !strings.Contains(zh, "JSON 解析失败") {
			t.Fatalf("unexpected zh: %s", zh)
		}
		if !strings.Contains(en, "Failed to parse manifest") {
			t.Fatalf("unexpected en: %s", en)
		}
	})

	t.Run("null json", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "plugin.json")
		if err := os.WriteFile(path, []byte("null"), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := ReadManifest(path)
		if err == nil {
			t.Fatal("expected error")
		}
		zh, en, ok := AsLocalized(err)
		if !ok {
			t.Fatalf("expected localized error, got %v", err)
		}
		if !strings.Contains(zh, "null") {
			t.Fatalf("unexpected zh: %s", zh)
		}
		if !strings.Contains(en, "must be a JSON object") {
			t.Fatalf("unexpected en: %s", en)
		}
	})
}

func TestAsLocalizedFallback(t *testing.T) {
	err := errors.New("plain")
	zh, en, ok := AsLocalized(err)
	if ok || zh != "plain" || en != "plain" {
		t.Fatalf("got ok=%v zh=%q en=%q", ok, zh, en)
	}
}
