// SiYuan community bazaar.
// Copyright (c) 2021-present, b3log.org
//
// Bazaar is licensed under Mulan PSL v2.
// You can use this software according to the terms and conditions of the Mulan PSL v2.
// You may obtain a copy of Mulan PSL v2 at:
//         http://license.coscl.org.cn/MulanPSL2
// THIS SOFTWARE IS PROVIDED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OF ANY KIND, EITHER EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT, MERCHANTABILITY OR FIT FOR A PARTICULAR PURPOSE.
// See the Mulan PSL v2 for more details.

package util

import (
	"fmt"
	"strings"
	"testing"

	"github.com/siyuan-note/bazaar/rules"
)

func TestParseReposFromBytes(t *testing.T) {
	t.Parallel()

	repos, err := ParseReposFromBytes("plugins.txt", []byte("alice/foo\nbob/bar\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 2 || repos[0] != "alice/foo" || repos[1] != "bob/bar" {
		t.Fatalf("repos=%v", repos)
	}

	_, err = ParseReposFromBytes("plugins.txt", []byte(" bad/line\n"))
	if err == nil {
		t.Fatal("want error for leading space")
	}
	if !strings.Contains(err.Error(), "plugins.txt") {
		t.Fatalf("error should mention file label: %v", err)
	}
}

func TestParseReposFromBytes_CRLF(t *testing.T) {
	t.Parallel()
	repos, err := ParseReposFromBytes("themes.txt", []byte("a/t1\r\nb/t2\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 2 {
		t.Fatalf("len=%d", len(repos))
	}
}

// 宽松模式（Stage / 弃用校验 / 巡检）：空行一律跳过，任何位置都不报错。
func TestParseReposFromBytes_BlankLinesLenient(t *testing.T) {
	t.Parallel()

	for _, data := range []string{
		"alice/foo\n\nbob/bar\n",
		"\nalice/foo\nbob/bar\n",
		"alice/foo\nbob/bar\n\n\n",
		"alice/foo\n \nbob/bar\n",
		"alice/foo\r\n\r\nbob/bar\r\n",
	} {
		repos, err := ParseReposFromBytes("plugins.txt", []byte(data))
		if err != nil {
			t.Fatalf("lenient parse of %q should succeed: %v", data, err)
		}
		if len(repos) != 2 || repos[0] != "alice/foo" || repos[1] != "bob/bar" {
			t.Fatalf("lenient parse of %q: repos=%v", data, repos)
		}
	}
}

// 严格模式（仅 PR Check）：末尾允许一个空行，其它位置的空行与纯空白行报错。
func TestParseReposFromBytes_BlankLinesStrict(t *testing.T) {
	t.Parallel()

	repos, err := ParseReposFromBytesStrict("plugins.txt", []byte("alice/foo\nbob/bar\n\n"))
	if err != nil {
		t.Fatalf("trailing blank line should be allowed: %v", err)
	}
	if len(repos) != 2 || repos[0] != "alice/foo" || repos[1] != "bob/bar" {
		t.Fatalf("repos=%v", repos)
	}

	cases := []struct {
		name string
		data string
		line int
	}{
		{"middle blank line", "alice/foo\n\nbob/bar\n", 2},
		{"leading blank line", "\nalice/foo\n", 1},
		{"two trailing blank lines", "alice/foo\n\n\n", 2},
		{"whitespace-only line", "alice/foo\n \nbob/bar\n", 2},
		{"CRLF middle blank line", "alice/foo\r\n\r\nbob/bar\r\n", 2},
	}
	for _, c := range cases {
		_, err = ParseReposFromBytesStrict("plugins.txt", []byte(c.data))
		if err == nil {
			t.Fatalf("%s: want error", c.name)
		}
		zh, en := rules.LocalizedMessages(err)
		want := fmt.Sprintf("第 %d 行", c.line)
		if !strings.Contains(zh, want) {
			t.Fatalf("%s: zh=%q should contain %q", c.name, zh, want)
		}
		if !strings.Contains(en, fmt.Sprintf("line %d", c.line)) {
			t.Fatalf("%s: en=%q should mention line %d", c.name, en, c.line)
		}
	}
}
