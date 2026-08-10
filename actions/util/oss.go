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
	"bytes"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/88250/gulu"
	"github.com/qiniu/go-sdk/v7/auth/qbox"
	"github.com/qiniu/go-sdk/v7/storage"
)

var (
	logger       = gulu.Log.NewLogger(os.Stdout)
	QINIU_BUCKET = os.Getenv("QINIU_BUCKET")
	QINIU_AK     = os.Getenv("QINIU_AK")
	QINIU_SK     = os.Getenv("QINIU_SK")

	ossUploadSem     chan struct{}
	ossUploadSemOnce sync.Once
)

func getOSSUploadSem() chan struct{} {
	ossUploadSemOnce.Do(func() {
		n := 16
		if s := os.Getenv("OSS_UPLOAD_CONCURRENCY"); s != "" {
			if parsed, err := strconv.Atoi(s); err == nil && parsed > 0 {
				n = parsed
			}
		}
		ossUploadSem = make(chan struct{}, n)
	})
	return ossUploadSem
}

func UploadOSS(ctx context.Context, key string, data []byte) (err error) {
	sem := getOSSUploadSem()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case sem <- struct{}{}:
	}
	defer func() { <-sem }()

	cfg := storage.Config{UseCdnDomains: true, UseHTTPS: true}
	mac := qbox.NewMac(QINIU_AK, QINIU_SK)
	bucketManager := storage.NewBucketManager(mac, &cfg)
	stat, err := bucketManager.Stat(QINIU_BUCKET, key)
	if err != nil {
		if !strings.Contains(err.Error(), "no such file or directory") {
			logger.Warnf("stat [%s] failed: %s", key, err)
		}
	} else if stat.Hash != "" {
		return nil
	}

	// 阻塞结束后检查是否已取消
	if err = ctx.Err(); err != nil {
		return err
	}

	putPolicy := storage.PutPolicy{
		Scope: fmt.Sprintf("%s:%s", QINIU_BUCKET, key), // overwrite if exists
	}

	formUploader := storage.NewFormUploader(&cfg)
	uploadToken := putPolicy.UploadToken(mac)
	if err = formUploader.Put(ctx, nil, uploadToken,
		key, bytes.NewReader(data), int64(len(data)), &storage.PutExtra{}); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		logger.Warnf("upload [%s] failed: %s, retry it", key, err)
		if err = formUploader.Put(ctx, nil, uploadToken,
			key, bytes.NewReader(data), int64(len(data)), &storage.PutExtra{}); err != nil {
			logger.Errorf("retry upload [%s] failed: %s", key, err)
			return err
		}
		logger.Infof("retry upload [%s] success", key)
	}
	return nil
}

const UserAgent = "bazaar/1.0.0 https://github.com/siyuan-note/bazaar"
