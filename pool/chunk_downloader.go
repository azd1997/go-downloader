/**********************************************************************
* @Author: Eiger (201820114847@mail.scut.edu.cn)
* @Date: 4/26/21 11:51 PM
* @Description: The file is for
***********************************************************************/

package pool

import (
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/azd1997/ego/edatabase"
)

const (
	DefaultCacheSize = 4096	// Byte	与task中的需对应起来
)

// Chunk 数据块
type Chunk struct {
	Begin int64 	// Begin/End是HTTP分块传输的RANGE范围，单位是Byte
	End int64

	Url    string
	DbPath string

	DataKey string
	TaskKey string

	tried int	// 已尝试多少次
}

func (c *Chunk) Valid() bool {
	return true
}

func NewChunkDownloader(id int, tryAgain chan <- *Chunk) *ChunkDownloader {
	return &ChunkDownloader{
		id:        id,
		status:    StatusIdle,
		client:    &http.Client{},
		tryAgain: tryAgain,
		cacheSize: DefaultCacheSize,
	}
}

// ChunkDownloader 块下载器
type ChunkDownloader struct {
	id int
	status Status
	client *http.Client

	tryAgain chan <- *Chunk	// 下载时遇到错误，就把下载任务（chunk）塞回,tryAgain就是cdp.chunkQueue
	cacheSize int			// 缓冲区大小，Byte
}

// Download 下载开始时状态变为busy，下载结束时变idle
func (cd *ChunkDownloader) Download(chunk *Chunk) error {

	var (
		req *http.Request
		rsp *http.Response
		err error
		buf []byte
		db edatabase.Database
		n int
		needSize int64
	)

	if chunk.tried > 10 {
		log.Fatalf(		// 程序退出
			"The (%d)th ChunkDownloader met error when download chunk. chunk={%d-%d,%s,%s}, err=%s\n",
			cd.id, chunk.Begin, chunk.End, chunk.Url, chunk.DbPath, "fail too much times")
	}

	req, err = http.NewRequest("GET", chunk.Url, nil)
	if err != nil {
		goto ERR
	}

	// 请求头中设置块下载的起止范围
	req.Header.Set(
		"Range",
		"bytes="+strconv.FormatInt(chunk.Begin, 10)+"-"+strconv.FormatInt(chunk.End, 10),
	)

	// 请求数据
	rsp, err = cd.client.Do(req)
	if err != nil {
		goto ERR
	}
	defer rsp.Body.Close()

	buf = make([]byte, cd.cacheSize)
	n, err = rsp.Body.Read(buf)
	// 检查下载的大小是否超出需要下载的大小
	// 这里End+1是因为http的Range的end是包括在需要下载的数据内的
	// 比如 0-1 的长度其实是2，所以这里end需要+1
	needSize = chunk.End + 1 - chunk.Begin
	if int64(n) > needSize {
		// 数据大小不正常
		// 一般是因为网络环境不好导致
		// 比如用中国电信下载国外文件

		// 设置数据大小来去掉多余数据
		// 并结束这个线程的下载
		n = int(needSize)
		err = io.EOF
	}
	if err != nil && err != io.EOF {
		goto ERR
	}

	// 将该分块数据写入数据库
	db, err = edatabase.OpenDatabase("badger", chunk.DbPath)
	if err != nil {
		goto ERR
	}
	defer db.Close()
	err = db.Set([]byte(chunk.DataKey), buf)
	if err != nil {
		goto ERR
	}
	// 确认写入成功后，将对应的任务删除
	err = db.Delete([]byte(chunk.TaskKey))
	if err != nil {
		goto ERR
	}

	return nil

ERR:
	log.Printf(
		"The (%d)th ChunkDownloader met error when download chunk. chunk={%d-%d,%s,%s}, err=%s\n",
		cd.id, chunk.Begin, chunk.End, chunk.Url, chunk.DbPath, err)

	chunk.tried++
	return nil
}



////////////////////////////////////////

// Status 下载器状态
type Status uint8

const (
	StatusIdle Status = iota
	StatusBusy
	// 其他(暂停等)?
)