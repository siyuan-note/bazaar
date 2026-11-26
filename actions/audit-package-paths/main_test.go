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
	"archive/zip"
	"bytes"
	"strings"
	"testing"

	"github.com/siyuan-note/bazaar/actions/util"
)

func TestCheckZIPPaths(t *testing.T) {
	for _, test := range []struct {
		name, entry, want string
		wrongDigest       bool
	}{
		{name: "valid package", entry: "script/components/TaskBoard.js"},
		{name: "invalid package", entry: `script\components\TaskBoard.js`, want: "paths use backslash"},
		{name: "wrong package bytes", entry: "script/components/TaskBoard.js", wrongDigest: true, want: "digest does not match"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var buffer bytes.Buffer
			archive := zip.NewWriter(&buffer)
			if _, err := archive.Create(test.entry); err != nil {
				t.Fatal(err)
			}
			if err := archive.Close(); err != nil {
				t.Fatal(err)
			}
			data := buffer.Bytes()
			digest := util.SHA256Hex(data)
			if test.wrongDigest {
				digest = util.SHA256Hex(nil)
			}
			err := checkZIPPaths(data, digest)
			if test.want == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected %q, got %v", test.want, err)
			}
		})
	}
}
