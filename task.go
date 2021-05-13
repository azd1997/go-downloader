package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"
)

func NewTask(url string, file *os.File, size int64) (*Task, error) {
	if size <= 0 {
		// 获取文件信息
		rsp, err := http.Head(url)
		if err != nil {
			fmt.Println(rsp, err)
			return nil, err
		}
		defer rsp.Body.Close()
		fmt.Println(rsp)
		fmt.Println(rsp.Header.Get("Content-Length"))
		fmt.Println(rsp.Header.Get("Accept-Ranges"))
		fmt.Println(rsp.TransferEncoding)

		size = rsp.ContentLength
	}
	return &Task{
		Url:url,
		Size: size,
		File: file,
		MaxThread: 5,
		CacheSize: 1024,
		BlockList: []Block{},
	}, nil
}

// Block 文件的下载块，用于断点续传
type Block struct {
	Begin int64 `json:"begin"`
	End int64 `json:"end"`
}

// Status 下载状态（速度、已下载的数据总量）
type Status struct {
	Speed int64		// B/s
	Downloaded int64
}

//

// Task 描述单个下载任务
// 对于http url指向的较大的文件，可以将之分块，并行下载
// Task -> 若干Block
type Task struct {
	Url string	// 文件url
	Size int64 	// 文件大小
	File *os.File // 写入的文件对象

	MaxThread int // 最大的并发数
	CacheSize int64 // 缓冲区大小

	BlockList []Block	// 文件分块（任务）

	onStart  func()
	onPause  func()
	onResume func()
	onDelete func()
	onFinish func()
	onError  func(int, error)

	paused bool
	completed bool
	status Status
}

func (t *Task) Start() {
	go func() {
		// 对任务进行切分
		t.shardBlock()
		// 任务开始时的动作
		t.do(t.onStart)
		// 开始统计下载速度
		t.startGetSpeeds()
		// 任务开始下载
		err := t.download()
		if err != nil {
			t.doOnError(0, err)
			return
		}
	}()
}

// shardBlock 任务分块
func (t *Task) shardBlock() {
	if t.Size <= 0 {
		t.BlockList = append(t.BlockList, Block{0, -1})
	} else {
		blockSize := t.Size / int64(t.MaxThread)
		begin := int64(0)
		// 数据平均分配给各个线程
		for i := 0; i < t.MaxThread; i++ {
			end := (int64(i) + 1) * blockSize
			t.BlockList = append(t.BlockList, Block{begin, end})
			begin = end + 1
		}
		// 将余出数据分配给最后一个线程
		t.BlockList[t.MaxThread-1].End += t.Size - t.BlockList[t.MaxThread-1].End
	}
}

// download 根据设置的分块数并发下载
func (t *Task) download() error {
	ok := make(chan bool, t.MaxThread)
	for i := range t.BlockList {
		go func(id int) {
			defer func() {
				ok <- true
			}()

			// 循环下载，直至将该分块下载完成，或者因为onError的特殊处理而退出
			for {
				err := t.downloadBlock(id)
				if err != nil {
					t.doOnError(0, err)
					// 重新下载
					continue
				}
				break
			}
		}(i)
	}

	// 等待全部下载完成
	for i := 0; i < t.MaxThread; i++ {
		<-ok
	}
	// 检查是否为暂停导致的“下载完成”
	if t.paused {
		t.do(t.onPause)		// 执行下载暂停时的动作
		return nil
	}
	// 确认全部分块下载完成，执行下载完成的动作
	t.completed = true
	t.do(t.onFinish)

	return nil
}

// downloadBlock 文件块下载器
// 根据分块id获取下载块的起始位置
func (t *Task) downloadBlock(id int) error {
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

// startGetSpeeds 统计1s内下载数据总量的变化
func (t *Task) startGetSpeeds() {
	go func() {
		var old = t.status.Downloaded
		for {
			if t.paused {
				t.status.Speed = 0
				return
			}
			time.Sleep(time.Second * 1)
			t.status.Speed = t.status.Downloaded - old
			old = t.status.Downloaded
		}
	}()
}

// GetStatus 获取下载统计信息
func (t *Task) GetStatus() Status {
	return t.status
}

// Pause 暂停下载
func (t *Task) Pause() {
	t.paused = true
}

// Resume 继续下载
func (t *Task) Resume() {
	t.paused = false
	go func() {
		if t.BlockList == nil {
			t.doOnError(0, errors.New("BlockList == nil, can not get block info"))
			return
		}

		t.do(t.onResume)
		err := t.download()
		if err != nil {
			t.doOnError(0, err)
			return
		}
	}()
}

// OnStart 任务开始时触发的事件
func (t *Task) OnStart(fn func()) *Task {
	t.onStart = fn
	return t
}

// OnPause 任务暂停时触发的事件
func (t *Task) OnPause(fn func()) *Task {
	t.onPause = fn
	return t
}

// OnResume 任务继续时触发的事件
func (t *Task) OnResume(fn func()) *Task {
	t.onResume = fn
	return t
}

// OnFinish 任务完成时触发的事件
func (t *Task) OnFinish(fn func()) *Task {
	t.onFinish = fn
	return t
}

// OnError 任务出错时触发的事件
// errCode为错误码，err为错误描述
func (t *Task) OnError(fn func(int, error)) *Task {
	t.onError = fn
	return t
}

// do 异步执行传入的fn
func (t *Task) do(fn func()) {
	if fn != nil {
		go fn()
	}
}

// doOnError 调用t.onError函数处理遇到的错误
func (t *Task) doOnError(errCode int, err error) {
	if t.onError != nil {
		go t.onError(errCode, err)
	}
}

