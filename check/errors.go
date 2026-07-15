// SiYuan community bazaar.
// Copyright (c) 2021-present, b3log.org
//
// Bazaar is licensed under Mulan PSL v2.
// You can use this software according to the terms and conditions of the Mulan PSL v2.
// You may obtain a copy of Mulan PSL v2 at:
//         http://license.coscl.org.cn/MulanPSL2
// THIS SOFTWARE IS PROVIDED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OF ANY KIND, EITHER EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT, MERCHANTABILITY OR FIT FOR A PARTICULAR PURPOSE.
// See the Mulan PSL v2 for more details.

package check

import "errors"

// LocalizedError 携带中英文错误消息；实现 error，Error() 返回英文（供日志与 %w 链）。
type LocalizedError struct {
	MessageZh string
	MessageEn string
	Cause     error
}

func (e *LocalizedError) Error() string {
	return e.MessageEn
}

func (e *LocalizedError) Unwrap() error {
	return e.Cause
}

// LocalizedErr 构造双语错误；cause 可选，供 errors.Is / Unwrap 使用。
func LocalizedErr(zh, en string, cause error) error {
	return &LocalizedError{MessageZh: zh, MessageEn: en, Cause: cause}
}

// AsLocalized 若 err 为 *LocalizedError（或包裹链中的），返回中英文消息。
func AsLocalized(err error) (zh, en string, ok bool) {
	if le, ok := errors.AsType[*LocalizedError](err); ok {
		return le.MessageZh, le.MessageEn, true
	}
	if err != nil {
		s := err.Error()
		return s, s, false
	}
	return "", "", false
}

// LocalizedMessages 返回 err 的中英文消息；非 LocalizedError 时中英文相同。
func LocalizedMessages(err error) (zh, en string) {
	zh, en, _ = AsLocalized(err)
	return zh, en
}

// IssueFromErr 将双语错误转为 Issue；非 LocalizedError 时中英文均使用 err.Error()。
func IssueFromErr(err error) Issue {
	zh, en, ok := AsLocalized(err)
	if ok {
		return issue(zh, en)
	}
	s := err.Error()
	return issue(s, s)
}
