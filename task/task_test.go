package task

import (
	"testing"

	"github.com/azd1997/blockchair_downloader/pool"
)

func TestTask(t *testing.T) {
	// 初始化下载器池
	err := pool.Init(100)
	if err != nil {
		t.Error(err)
	}
	pool.Start()
	defer pool.Stop()

	// 创建任务
	url := "https://gz.blockchair.com/bitcoin/inputs/blockchair_bitcoin_inputs_20120425.tsv.gz"
	task, err := NewTask(url)
	if err != nil {
		t.Error(err)
	}

	// 开始任务
	err = task.Start()
	if err != nil {
		t.Error(err)
	}
}
