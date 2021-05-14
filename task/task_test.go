package task

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/azd1997/blockchair_downloader/pool"
)

var (
	// 20100101 压缩文件约1KB， 只会分一个块下载，下载完成后直接就可用
	url_FileSmallerThan4KB = "https://gz.blockchair.com/bitcoin/inputs/blockchair_bitcoin_inputs_20100101.tsv.gz"
	// 20100404 约19KB，分5个块
	url_FileLargerThan4KB = "https://gz.blockchair.com/bitcoin/inputs/blockchair_bitcoin_inputs_20100404.tsv.gz"
	// 20120425 约32MB，用于测试下载器的下载速度
	url_FileMuchLargerThan4KB = "https://gz.blockchair.com/bitcoin/inputs/blockchair_bitcoin_inputs_20120425.tsv.gz"

	//
	url_Video_1Mb = "https://sample-videos.com/video123/mp4/720/big_buck_bunny_720p_1mb.mp4"
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
	url := url_FileMuchLargerThan4KB
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

func TestTask_CompareWithDownloadDirectly(t *testing.T) {
	// 选择一个体积较小的文件下载

	//url := url_Video_1Mb
	//urlFile := DownloadDir + "SampleVideo_1280x720_1mb.mp4"

	url := url_FileLargerThan4KB
	urlFile := DownloadDir + "Sample_blockchair_bitcoin_inputs_20100404.tsv.gz"

	data1 := download1(url)
	//data2 := download2(url)
	data2 := localFile(urlFile)
	if !bytes.Equal(data1, data2) {
		t.Error("data1 != data2")
		t.Logf("len(data1)=%d, len(data2)=%d\n", len(data1), len(data2))
	}
	length := len(data1)
	// 对data1,data2每16B为一行进行展示
	unit := 16
	for i:=0; ; i++ {
		begin := i*unit
		if begin > length -1 {
			break
		}
		end := begin + unit -1
		if end > length-1 {
			end = length-1
		}
		fmt.Printf("data1[%d:%d]: %v\n", begin, end+1, data1[begin:end+1])
		fmt.Printf("data2[%d:%d]: %v\n", begin, end+1, data2[begin:end+1])
	}
	//fmt.Println(data1)
	//fmt.Println(data2)
}

// 把下载的内容全部返回
func download1(url string) []byte {
	// 初始化下载器池
	err := pool.Init(100)
	if err != nil {
		panic(err)
	}
	pool.Start()
	defer pool.Stop()

	// 创建任务
	task, err := NewTask(url)
	if err != nil {
		panic(err)
	}

	// 开始任务
	err = task.Start()
	if err != nil {
		panic(err)
	}

	// 把文件中内容读出来
	data := make([]byte, task.FileSize)
	f, err := os.Open(task.FileName)
	if err != nil {
		panic(err)
	}
	//io.Copy(bytes.NewBuffer(data), f)
	f.Read(data)
	return data
}

func download2(url string) []byte {
	rsp, err := http.Get(url)
	if err != nil {
		panic(err)
	}
	fmt.Println(rsp.Header)
	data := make([]byte, rsp.ContentLength)
	//io.Copy(bytes.NewBuffer(data), rsp.Body)
	fmt.Println("xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx")
	rsp.Body.Read(data)
	return data
}

func localFile(filepath string) []byte {
	stat, err := os.Stat(filepath)
	if err != nil {
		panic(err)
	}
	size := stat.Size()

	data := make([]byte, size)
	fmt.Println("xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx")

	f, err := os.Open(filepath)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	_, err = f.Read(data)
	if err != nil {
		return nil
	}

	return data
}