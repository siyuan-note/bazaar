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
	"strings"
	"testing"
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
