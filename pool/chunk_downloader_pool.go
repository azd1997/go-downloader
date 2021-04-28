/**********************************************************************
* @Author: Eiger (201820114847@mail.scut.edu.cn)
* @Date: 4/26/21 11:55 PM
* @Description: The file is for
***********************************************************************/

package pool

import (
	"errors"
	"sync"
)

var cdpool = &sync.Pool{
	New: func() interface{} {
		return new(ChunkDownloader)
	},
}

func Get() *ChunkDownloader {
	if get := cdpool.Get(); get == nil {
		return nil
	} else {
		return get.(*ChunkDownloader)
	}
}

func Return(cd *ChunkDownloader) {
	if cd != nil {
		cdpool.Put(cd)
	}
}

// ChunkDownloaderPool 下载器池
// 不管程序中有多少块下载任务，调用CDP的下载方法
type ChunkDownloaderPool struct {
	chunkQueue []*ChunkDownloader	// 末尾是空闲的，前头是在使用的
	maxChunkDownloader int	// 最多支持多少个ChunkDownloader

}

var cdp *ChunkDownloaderPool

func Init(max int) error {
	if max <= 0 {
		return errors.New("cdp need at least 1 cd")
	}
	cdp = &ChunkDownloaderPool{
		chunkQueue:         make([]*ChunkDownloader, 0, max),
		maxChunkDownloader: max,
	}
	return nil
}

// DownloadChunk 外部调用，将块下载任务添加到
func (cdp *ChunkDownloaderPool) DownloadChunk(chunk Chunk) {

}

// Start 启动调度循环
func (cdp *ChunkDownloaderPool) Start() {

}

// Stop 关闭调度循环，清除所有结构
func (cdp *ChunkDownloaderPool) Stop() {

}
