/**********************************************************************
* @Author: Eiger (201820114847@mail.scut.edu.cn)
* @Date: 4/26/21 11:51 PM
* @Description: The file is for
***********************************************************************/

package pool

import (
	"io"
	"net/http"
	"os"
	"strconv"
)

// Chunk 数据块
type Chunk struct {
	Begin int64 `json:"begin"`	// Begin/End是HTTP分块传输的RANGE范围，单位是Byte
	End int64 `json:"end"`

	Url string
	File *os.File
}

// ChunkDownloader 块下载器
type ChunkDownloader struct {
	client *http.Client

}



func (cd *ChunkDownloader) Download() {
	request, err := http.NewRequest("GET", t.Url, nil)
	if err != nil {
		return err
	}
	begin := t.BlockList[id].Begin
	end := t.BlockList[id].End
	if end != -1 {	// end=-1则直接整个文件下载，不分块
		// 请求头中设置块下载的起止范围
		request.Header.Set(
			"Range",
			"bytes="+strconv.FormatInt(begin, 10)+"-"+strconv.FormatInt(end, 10),
		)
	}

	// 请求数据
	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var buf = make([]byte, t.CacheSize)
	for {
		if t.paused == true {
			// 下载暂停
			return nil
		}

		n, e := resp.Body.Read(buf)

		bufSize := int64(len(buf[:n]))
		if end != -1 {
			// 检查下载的大小是否超出需要下载的大小
			// 这里End+1是因为http的Range的end是包括在需要下载的数据内的
			// 比如 0-1 的长度其实是2，所以这里end需要+1
			needSize := t.BlockList[id].End + 1 - t.BlockList[id].Begin
			if bufSize > needSize {
				// 数据大小不正常
				// 一般是因为网络环境不好导致
				// 比如用中国电信下载国外文件

				// 设置数据大小来去掉多余数据
				// 并结束这个线程的下载
				bufSize = needSize
				n = int(needSize)
				e = io.EOF
			}
		}
		// 将缓冲数据写入硬盘
		t.File.WriteAt(buf[:n], t.BlockList[id].Begin)

		// 更新已下载大小
		t.status.Downloaded += bufSize
		t.BlockList[id].Begin += bufSize

		if e != nil {
			if e == io.EOF {
				// 数据已经下载完毕
				return nil
			}
			return e
		}
	}

	return nil
}