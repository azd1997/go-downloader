/**********************************************************************
* @Author: Eiger (201820114847@mail.scut.edu.cn)
* @Date: 4/26/21 11:55 PM
* @Description: The file is for
***********************************************************************/

package pool

import (
	"container/heap"
	"errors"
	"log"
	"sync"
)

var (
	ErrMaxDownloader = errors.New("max downloader number")
)

var (
	cdp *ChunkDownloaderPool
	notifies map[string]chan<-struct{}
)

func Init(max int) error {
	if max <= 0 {
		return errors.New("cdp need at least 1 cd")
	}
	cdp = &ChunkDownloaderPool{
		busyChunkDownloaderMap: map[int]*ChunkDownloader{},
		idleChunkDownloaderMap: map[int]*ChunkDownloader{},
		maxChunkDownloader: max,
		curHighest: -1,	// 表示没有可用的
		idHeap: InitRankHeap(),
		chunkQueue: make(chan *Chunk, 100),
		stop: make(chan struct{}),
	}
	notifies = map[string]chan<- struct{}{}
	return nil
}

func Start() {
	if cdp != nil {
		cdp.Start()
	}
}

func Stop() {
	if cdp != nil {
		cdp.Stop()
	}
}

func Download(chunk Chunk) {
	cdp.DownloadChunk(chunk)
}

func RegisterNotify(task string, notify chan <- struct{}) {
	notifies[task] = notify
}

func RemoveNotify(task string) {
	delete(notifies, task)
}

// ChunkDownloaderPool 下载器池
// 不管程序中有多少块下载任务，调用CDP的下载方法
type ChunkDownloaderPool struct {

	busyChunkDownloaderMap map[int]*ChunkDownloader
	idleChunkDownloaderMap map[int]*ChunkDownloader
	sync.RWMutex	// 两个表基本都需要同时使用，所以只用一把锁
	maxChunkDownloader int	// 最多支持多少个ChunkDownloader
	curHighest int	// 最高的下载器id(0-)		// 用于为下载器分配递增id
	idHeap *RankHeap	//

	chunkQueue chan *Chunk	// 下载任务队列

	stop chan struct{}	// 如果没有初始化stop，对nil stop的写操作将阻塞
}

// DownloadChunk 外部调用，将块下载任务添加到
func (cdp *ChunkDownloaderPool) DownloadChunk(chunk Chunk) {
	if chunk.Valid() {
		cdp.chunkQueue <- &chunk
	}
}

// Start 启动调度循环
func (cdp *ChunkDownloaderPool) Start() {
	go func() {
		for {
			select {
			case chunk := <- cdp.chunkQueue:
				go cdp.download(chunk)
			case <- cdp.stop:
				return
			}
		}
	}()
}

// 调用时应 go cdp.download()
func (cdp *ChunkDownloaderPool) download(chunk *Chunk) {

	///////////////// 获取下载器 ////////////////////
	cd, cdr, err := cdp.getChunkDownloader()
	if err != nil && err != ErrMaxDownloader {
		log.Fatalln("ChunkDownloaderPool.download fail: ", err)
	}
	if err != nil && err == ErrMaxDownloader {	// 将下载任务重新塞回
		cdp.chunkQueue <- chunk
		return
	}
	if cd == nil || cdr == nil {
		log.Fatalln("ChunkDownloaderPool.download fail: unexpected nil ChunkDownloader")
	}

	///////////////// 下载 ////////////////////
	err = cd.Download(chunk)
	if err != nil {
		log.Fatalln("ChunkDownloaderPool.download fail: ", err)
	}
	// 下载成功后通知Task
	if notify, ok := notifies[chunk.Url]; ok && notify != nil {
		notify <- struct{}{}
	}

	///////////////// 归还下载器 ////////////////////

	cdp.retChunkDownloader(cd, cdr)

	log.Printf("ChunkDownloaderPool.download succ: chunk={%d-%d,%s}\n",
		chunk.Begin, chunk.End, chunk.Url)
}

// 获取下载器时，要将idle下载器转变为busy
func (cdp *ChunkDownloaderPool) getChunkDownloader() (*ChunkDownloader, *chunkDownloaderRank, error) {
	cdp.Lock()
	defer cdp.Unlock()

	var cd *ChunkDownloader
	var cdr *chunkDownloaderRank

	// 1.1 如果有空闲下载器，则直接取用
	if idles := len(cdp.idleChunkDownloaderMap); idles > 0 {
		// 从idHeap头部取出一个idle下载器的标识
		top := cdp.idHeap.Pop()
		if top == nil || top.status != StatusIdle {
			log.Fatalln("ChunkDownloaderPool.download fail: top == nil || top.status != StatusIdle")
		}
		// 取出对应的下载器
		topCD, exists := cdp.idleChunkDownloaderMap[top.chunkDownloaderId]
		if !exists || topCD.status != StatusIdle {
			log.Fatalln("ChunkDownloaderPool.download fail: !exists || topCD.status != StatusIdle")
		}
		// 将该下载器从原处取下
		delete(cdp.idleChunkDownloaderMap, top.chunkDownloaderId)

		cd = topCD
		cdr = top
	}

	// 1.2 如果没有空闲的下载器，试图创建
	if len(cdp.busyChunkDownloaderMap) >= cdp.maxChunkDownloader {	// 不能再创建
		return nil, nil, ErrMaxDownloader
	}

	// 1.3 创建新下载器
	cd = NewChunkDownloader(cdp.curHighest + 1, cdp.chunkQueue)
	cdp.curHighest++
	cdr = &chunkDownloaderRank{
		chunkDownloaderId: cd.id,
		status:            StatusIdle,
	}

	// 2 将取出的下载器添加到busy表
	cdp.busyChunkDownloaderMap[cd.id] = cd
	cdr.status = StatusBusy
	cdp.idHeap.Push(cdr)

	return cd, cdr, nil
}

// 归还下载器时，要将busy下载器转变为idle
func (cdp *ChunkDownloaderPool) retChunkDownloader(cd *ChunkDownloader, cdr *chunkDownloaderRank) {
	cdp.Lock()
	defer cdp.Unlock()

	delete(cdp.busyChunkDownloaderMap, cd.id)
	cdp.idleChunkDownloaderMap[cd.id] = cd
	cdr.status = StatusIdle
	cdp.idHeap.Heapify()	// 让cdr到达正确的位置
}


// Stop 关闭调度循环，清除所有结构
func (cdp *ChunkDownloaderPool) Stop() {
	cdp.stop <- struct{}{}
	log.Println("ChunkDownloaderPool.Stop")
}

///////////////////////// 堆,用来对cd进行排序 ///////////////////////////

func InitRankHeap() *RankHeap {
	rh := &rankHeap{}
	heap.Init(rh)
	return &RankHeap{
		rankHeap:*rh,
	}
}

type RankHeap struct {
	rankHeap
	sync.Mutex
}

// Heapify 重新堆化
// 不使用heap.Fix(i)的原因是，没有记录元素的下标
func (rh *RankHeap) Heapify() {
	rh.Lock()
	heap.Init(&rh.rankHeap)
	rh.Unlock()
}

func (rh *RankHeap) Push(r *chunkDownloaderRank) {
	if r == nil {
		return
	}
	rh.Lock()
	heap.Push(&rh.rankHeap, r)
	rh.Unlock()
}

func (rh *RankHeap) Pop() *chunkDownloaderRank {
	var ret interface{}
	rh.Lock()
	ret = heap.Pop(&rh.rankHeap)
	rh.Unlock()
	pop, ok := ret.(*chunkDownloaderRank)
	if !ok {
		return nil
	}
	return pop
}

type chunkDownloaderRank struct {
	chunkDownloaderId int
	status Status
}

// rankHeap 将status小(idle)、id序号小的排在堆顶
type rankHeap []*chunkDownloaderRank

func (h rankHeap) Len() int           { return len(h) }
func (h rankHeap) Less(i, j int) bool {
	if h[i].status == h[j].status {
		return h[i].chunkDownloaderId < h[j].chunkDownloaderId
	}
	return h[i].status < h[j].status
}
func (h rankHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *rankHeap) Push(x interface{}) {
	// Push and Pop use pointer receivers because they modify the slice's length,
	// not just its contents.
	*h = append(*h, x.(*chunkDownloaderRank))
}

func (h *rankHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}


