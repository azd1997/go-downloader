package task

import (
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/azd1997/blockchair_downloader/edb"
	"github.com/azd1997/blockchair_downloader/pool"
	"github.com/azd1997/ego/utils"
)

const (
	DefaultChunkSize = 20 * 1024

	KeyLength = 17 // 1+8+8
	TaskKeyPrefix = 'T'
	DataKeyPrefix = 'D'
	NumKeyPrefix = 'N'
	PlaceHolder = '-'

	DownloadDir = "./download/"
)

// Task 任务
// 一个Task描述一个下载文件任务url等相关状态
// 并发：将大文件拆分为众多小分块进行http
// 断点续传：所有分片通过BadgerDB存储，全下载完成后拼接成完整文件
type Task struct {
	Url            string `json:"url"`             // 下载url
	UrlHash        string `json:"url_hash"`        // url的哈希值
	FileSize       int64  `json:"file_size"`       // 文件大小
	ChunkSupported bool   `json:"chunk_supported"` // 是否支持HTTP分块传输
	FileName       string `json:"file_name"`       // 文件名

	// 切分
	ChunkSize int64 `json:"chunk_size"` // 标准的分块大小，1024倍数 暂设为4096
	ChunkNum  int64 `json:"chunk_num"`  // 总共的分块数量
	ChunkLeft int64	`json:"chunk_left"`// 剩下的分块数
	Resume bool `json:"resume"` // 续传

	StartTime time.Time `json:"start_time"`	// 开始时间

	// 数据库
	// 三种键：数据、任务、数量
	// 数据键格式：D|[start][end]
	// 任务键格式：T|[start][end]
	// 数量键：N
	DbPath string `json:"db_path"`
	db edb.DB	// 数据库连接实例

	notify chan struct{}	// 用于向pool注册，当该Task所有分块下载完成后通知该Task
	close chan struct{} // 该任务结束
}

//func (t *Task) DecrChunkLeft() {
//	atomic.AddInt64(&t.chunkLeft, -1)
//}
//
//func (t *Task) LoadChunkLeft() int64 {
//	return atomic.LoadInt64(&t.chunkLeft)
//}


// NewTask 新建任务
func NewTask(url string) (*Task, error) {

	task := &Task{
		Url: url,
		ChunkSize: DefaultChunkSize,
		StartTime: time.Now(),
		notify: make(chan struct{}),
		close: make(chan struct{}),
	}
	//fmt.Println("hex(md5(url)) = ", task.UrlHash)

	// 获取url哈希值（有些url特别长，所以用哈希值替代比较合适）
	h := md5.Sum([]byte(url))
	task.UrlHash = hex.EncodeToString(h[:])
	//fmt.Println("hex(md5(url)) = ", task.UrlHash)

	// 获取文件大小以及是否支持按字节分块传输
	rsp, err := http.Head(url)
	if err != nil {
		//fmt.Println(rsp, err)
		return nil, err
	}
	task.FileSize = rsp.ContentLength
	task.ChunkSupported = rsp.Header.Get("Accept-Ranges") == "bytes" // 这表示服务端支持按字节下载
	rsp.Body.Close()
	//fmt.Println("task.fileSize = ", task.FileSize)
	//fmt.Println("task.shardSupported = ", task.ChunkSupported)

	// 确定分块数量
	if task.ChunkSupported {
		task.ChunkNum = task.FileSize / task.ChunkSize
		if task.FileSize % task.ChunkSize != 0 {
			task.ChunkNum++
		}
	}

	// 检查下载目录是否存在
	if exists, _ := utils.DirExists(DownloadDir); !exists {
		err = os.MkdirAll(DownloadDir, 0777)
		if err != nil {
			return nil, err
		}
	}

	// 文件名 确定下载的唯一文件名，避免文件名重复
	strs := strings.Split(url, "/")
	fileName := strs[len(strs)-1]
	if strings.TrimSpace(fileName) == "" {
		fileName = task.UrlHash
	}
	// 本地如果已经有该文件名的文件，将该文件名追加日期
	if exist, err := utils.FileExists(fileName); exist || err != nil {
		fileName = fileName + "-" + strconv.Itoa(int(time.Now().Unix()))
	}
	fileName = DownloadDir + fileName
	task.FileName = fileName
	//fmt.Println("task.fileName = ", task.FileName)

	if task.ChunkSupported {
		// 如果本地已经有对应数据库，说明是续传；否则根据fileSize分块，并写入数据库
		dbPath := fileName + ".DOWNLOADING"
		task.DbPath = dbPath
		//fmt.Println("task.dbPath = ", task.FileName)
		// 打开数据库（如果没有就创建）
		if edb.DbExists(dbPath) {
			task.Resume = true
		}

		// 创建数据库连接实例，直到任务结束或程序停止才关闭
		db, err := edb.OpenEDB(task.DbPath)
		if err != nil {
			return nil, err
		}
		task.db = db
	}

	// 打印信息
	fmt.Println()
	fmt.Println("==================== Task Info ====================")
	fmt.Println(utils.JsonMarshalIndentToString(task))
	fmt.Println()

	return task, nil
}

// Start 开始下载任务
func (t *Task) Start() error {
	if t.ChunkSupported {
		return t.downloadChunkly()
	}
	return t.downloadDirectly()
}

// 直接下载（不支持分块下载的情况）
func (t *Task) downloadDirectly() error {
	rsp, err := http.Get(t.Url)
	if err != nil {
		return err
	}
	f, err := os.Create(t.FileName)
	if err != nil {
		return err
	}
	io.Copy(f, rsp.Body)
	return nil
}

// 分块下载
func (t *Task) downloadChunkly() error {
	if t.db == nil {
		return errors.New("nil db instance")
	}

	// 向cdp注册一个通知通道
	pool.RegisterNotify(t.Url, t.notify)

	// 读取或添加所有分块任务
	chunks := make([]*pool.Chunk, 0)
	if t.Resume {
		t.db.IterKey(func(k []byte) error {	// 符合条件的k就是任务 注意k最好拷贝后使用
			if len(k) == 0 {
				return errors.New("nil key")
			}
			typ := k[0]
			if typ == TaskKeyPrefix {
				if len(k) != KeyLength {	// 1 + 8 + 8
					return errors.New("error task key format: length should = 17")
				}
				begin, end := int64(-1), int64(-1)
				begin, _ = binary.Varint(k[1:9])
				end, _ = binary.Varint(k[9:17])
				if end < begin {
					return errors.New("error task key format: begin should <= end")
				}
				t.ChunkLeft++	// 设置还剩下的任务数
				// 对于一个符合条件的任务，要丢给下载池下载
				dkstr := string(k)
				dk := []byte(dkstr)
				dk[0] = DataKeyPrefix
				dkstr = string(dk)
				chunks = append(chunks, &pool.Chunk{
					Begin:  begin,
					End:    end,
					Url:    t.Url,
					Db: t.db,
					TaskKey: string(k),
					DataKey: dkstr,
				})
			}
			return nil
		})
	} else {
		t.ChunkLeft = t.ChunkNum	// 设置还剩下的任务数
		// 初次下载，需要划分任务
		for i:=int64(1); i<=t.ChunkNum; i++ {
			// 计算begin,end
			begin := (i-1) * t.ChunkSize
			end := begin + t.ChunkSize - 1
			if end > t.FileSize - 1 {
				end = t.FileSize - 1
			}
			// 构建key并存储
			key := make([]byte, KeyLength)
			key[0] = TaskKeyPrefix
			binary.PutVarint(key[1:9], begin)
			binary.PutVarint(key[9:17], end)
			if err := t.db.Set(key, []byte{PlaceHolder}); err != nil {
				return err
			}
			// 将分块任务发给cdp
			dkstr := string(key)
			dk := []byte(dkstr)
			dk[0] = DataKeyPrefix
			dkstr = string(dk)
			chunks = append(chunks, &pool.Chunk{
				Begin:  begin,
				End:    end,
				Url:    t.Url,
				Db: t.db,
				TaskKey: string(key),
				DataKey: dkstr,
			})
		}
	}

	if len(chunks) == 0 {
		t.db.Close()
		log.Println("no chunks need to download")
		return nil
	}

	// 下载
	for i:=0; i<len(chunks); i++ {
		pool.Download(*(chunks[i]))
	}


	// 得到数据库中的分块任务（无论是续传还是初传），这些任务需要传给下载器池cdp
	// cdp下载分块完成后将数据库中任务删除，内容写入
	// 数据库中所有分块任务结束后，任务下载完成

	// 等待所有分块下载完成
	for {
		select {
		case <-t.notify: // 一个分块下载结束
			t.ChunkLeft--
			log.Printf("Task(%s): downloaded (%d/%d) elapsed %s\n",
				t.Url, (t.ChunkNum - t.ChunkLeft), t.ChunkNum, time.Now().Sub(t.StartTime).String())
			if t.ChunkLeft == 0 {
				pool.RemoveNotify(t.Url)
				err := t.mergeChunksToFile()	// 合并文件
				if err != nil {
					return err
				}
				// 关闭数据库
				t.db.Close()
				return nil
			}
		case <-t.close:
			log.Printf("Task(%s): downloaded (%d/%d) elapsed %s. quit unexpectly\n",
				t.Url, (t.ChunkNum - t.ChunkLeft), t.ChunkNum, time.Now().Sub(t.StartTime).String())
			// 关闭数据库
			t.db.Close()
			return nil
		}
	}
}

func (t *Task) mergeChunksToFile() error {
	// 创建文件
	f, err := os.Create(t.FileName)
	if err != nil {
		return err
	}
	defer f.Close()

	// 拼接所有分块
	for i:=int64(1); i<=t.ChunkNum; i++ {
		// 计算begin,end
		begin := (i - 1) * t.ChunkSize
		end := begin + t.ChunkSize - 1
		if end > t.FileSize-1 {
			end = t.FileSize - 1
		}
		// 构建key
		key := make([]byte, KeyLength)
		key[0] = DataKeyPrefix
		binary.PutVarint(key[1:9], begin)
		binary.PutVarint(key[9:17], end)
		// 查询对应数据
		v, err := t.db.Get(key)
		if err != nil {
			return err
		}
		// 写入文件
		_, err = f.WriteAt(v, begin)
		if err != nil {
			return err
		}
	}

	// 打印文件信息
	stat, _ := f.Stat()
	fmt.Printf("文件大小：%d byte\n", stat.Size())
	return nil
}

func (t *Task) mergeChunksToFile2() error {
	// 创建文件
	f, err := os.Create(t.FileName)
	if err != nil {
		return err
	}
	defer f.Close()

	// 拼接所有分块
	for i:=int64(1); i<=t.ChunkNum; i++ {
		// 计算begin,end
		begin := (i - 1) * t.ChunkSize
		end := begin + t.ChunkSize - 1
		if end > t.FileSize-1 {
			end = t.FileSize - 1
		}
		// 构建key
		key := make([]byte, KeyLength)
		key[0] = DataKeyPrefix
		binary.PutVarint(key[1:9], begin)
		binary.PutVarint(key[9:17], end)
		// 查询对应数据
		v, err := t.db.Get(key)
		if err != nil {
			return err
		}
		// 写入文件
		_, err = f.WriteAt(v, begin)
		if err != nil {
			return err
		}
	}

	// 打印文件信息
	stat, _ := f.Stat()
	fmt.Printf("文件大小：%d byte\n", stat.Size())
	return nil
}
