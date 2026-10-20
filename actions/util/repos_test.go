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

func TestParseReposFromBytes_BlankLines(t *testing.T) {
	t.Parallel()

	// 末尾单个空行允许（默认情况的兜底）
	repos, err := ParseReposFromBytes("plugins.txt", []byte("alice/foo\nbob/bar\n\n"))
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
		_, err = ParseReposFromBytes("plugins.txt", []byte(c.data))
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
