package pool

import (
	"fmt"
	"testing"

	"github.com/azd1997/blockchair_downloader/edb"
)

func TestChunkDownloader_Download(t *testing.T) {

	// 准备好数据库
	dbPath := "./tmp.DOWNLOADING"
	db, err := edb.OpenEDB(dbPath)
	if err != nil {
		panic(err)
	}
	defer db.Close()	// 记得关闭
	dk, tk := "datakey", "taskkey"
	err = db.Set([]byte(tk), []byte("-"))
	if err != nil {
		panic(err)
	}


	// 块下载
	tryagain := make(chan *Chunk, 10)
	cd := NewChunkDownloader(1, tryagain)
	
	chunk := &Chunk{
		Begin:   0,
		End:     10000,
		Url:     "https://gz.blockchair.com/bitcoin/inputs/blockchair_bitcoin_inputs_20130425.tsv.gz",
		Db:  db,
		DataKey: dk,
		TaskKey: tk,
		tried:   0,
	}
	
	err = cd.Download(chunk)
	if err != nil {
		panic(err)
	}

	// 检查下载是否成功
	v, err := db.Get([]byte(dk))
	if err != nil {
		panic(err)
	}
	if len(v) != int(chunk.End + 1 - chunk.Begin) {
		panic("errrrr")
	}

	fmt.Println("success")
}
