// SiYuan community bazaar.
// Copyright (c) 2021-present, b3log.org
//
// Pipe is licensed under Mulan PSL v2.
// You can use this software according to the terms and conditions of the Mulan PSL v2.
// You may obtain a copy of Mulan PSL v2 at:
//         http://license.coscl.org.cn/MulanPSL2
// THIS SOFTWARE IS PROVIDED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OF ANY KIND, EITHER EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT, MERCHANTABILITY OR FIT FOR A PARTICULAR PURPOSE.
// See the Mulan PSL v2 for more details.

package util

import (
	"bytes"
	"context"
	"fmt"
	"os"

	"github.com/88250/gulu"
	"github.com/qiniu/go-sdk/v7/auth/qbox"
	"github.com/qiniu/go-sdk/v7/storage"
)

var logger = gulu.Log.NewLogger(os.Stdout)

func UploadOSS(key, contentType string, data []byte) (err error) {
	bucket := os.Getenv("QINIU_BUCKET")
	ak := os.Getenv("QINIU_AK")
	sk := os.Getenv("QINIU_SK")
	putPolicy := storage.PutPolicy{
		Scope: fmt.Sprintf("%s:%s", bucket, key), // overwrite if exists
	}
	cfg := storage.Config{Zone: &storage.ZoneHuadong, UseCdnDomains: true, UseHTTPS: true}
	formUploader := storage.NewFormUploader(&cfg)
	if err = formUploader.Put(context.Background(), nil, putPolicy.UploadToken(qbox.NewMac(ak, sk)),
		key, bytes.NewReader(data), int64(len(data)), &storage.PutExtra{MimeType: contentType}); nil != err {
		logger.Warnf("upload [%s] failed: %s, retry it", key, err)
		if err = formUploader.Put(context.Background(), nil, putPolicy.UploadToken(qbox.NewMac(ak, sk)),
			key, bytes.NewReader(data), int64(len(data)), &storage.PutExtra{MimeType: contentType}); nil != err {
			logger.Errorf("retry upload [%s] failed: %s", key, err)
			return
		}
		logger.Infof("retry upload [%s] success", key)
	}
	return
}
