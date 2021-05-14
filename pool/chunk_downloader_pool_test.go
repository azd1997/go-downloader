package pool

import (
	"fmt"
	"github.com/azd1997/blockchair_downloader/edb"
	"testing"
)

func TestRankHeap(t *testing.T) {
	rh := InitRankHeap()

	rh.Push(&chunkDownloaderRank{
		chunkDownloaderId: 1,
		status:            1,
	})

	rh.Push(&chunkDownloaderRank{
		chunkDownloaderId: 2,
		status:            0,
	})

	rh.Push(&chunkDownloaderRank{
		chunkDownloaderId: 3,
		status:            1,
	})

	rh.Push(&chunkDownloaderRank{
		chunkDownloaderId: 4,
		status:            0,
	})


	ids := [4]int{}
	for i:=0; i<4; i++ {
		ids[i] = rh.rankHeap[i].chunkDownloaderId
	}
	t.Log(ids)

	// 出来的顺序应该是 2 4 1 3
	out := [4]int{}
	for i:=0; i<4; i++ {
		top := rh.Pop()
		out[i] = top.chunkDownloaderId
	}

	if out != [4]int{2,4,1,3} {
		t.Error("error")
		t.Log(out)
		return
	}
	t.Log("succ")
}

func TestChunkDownloaderPool_OneChunk(t *testing.T) {
	err := Init(3)
	if err != nil {
		t.Error("Init fail: ", err)
	}

	Start()
	defer Stop()

	taskurl := "https://gz.blockchair.com/bitcoin/inputs/blockchair_bitcoin_inputs_20130425.tsv.gz"
	notify := make(chan struct{})
	RegisterNotify(taskurl, notify)

	// 准备好数据库
	dbPath := "./tmp.DOWNLOADING"
	db, err := edb.OpenEDB(dbPath)
	if err != nil {
		panic(err)
	}
	defer db.Close()
	dk, tk := "datakey", "taskkey"
	err = db.Set([]byte(tk), []byte("-"))
	if err != nil {
		panic(err)
	}

	chunk := &Chunk{
		Begin:   0,
		End:     10000,
		Url:     taskurl,
		Db:  db,
		DataKey: dk,
		TaskKey: tk,
		tried:   0,
	}

	Download(*chunk)

	// 检查下载是否成功
	v, err := db.Get([]byte(dk))
	if err != nil {
		panic(err)
	}
	if len(v) != int(chunk.End + 1 - chunk.Begin) {
		panic("errrrr")
	}

	<-notify
	fmt.Println("success")

	// 接下来就是defer Stop()
}