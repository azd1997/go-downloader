package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

func main(){
	file, err := os.Open("./tmp")
	if err != nil {
		panic(err)
	}
	defer file.Close()

// http://packages.linuxdeepin.com/ubuntu/dists/devel/main/binary-amd64/Packages.bz2
	// https://gz.blockchair.com/bitcoin/inputs/blockchair_bitcoin_inputs_20130425.tsv.gz
	url := "https://gz.blockchair.com/bitcoin/inputs/blockchair_bitcoin_inputs_20130425.tsv.gz"


	// 设置下载任务Task
	task, err := NewTask(url, file, -1)
	if err != nil {
		panic(err)
	}
	var exit = make(chan bool)
	var resume = make(chan bool)
	var pause bool
	var wg sync.WaitGroup
	wg.Add(1)
	task.OnStart(func() {
		fmt.Println("download started")
		format := "\033[2K\r%v/%v [%s] %v byte/s %v"
		for {
			status := task.GetStatus()
			var i = float64(status.Downloaded) / float64(task.Size) * 50
			h := strings.Repeat("=", int(i)) + strings.Repeat(" ", 50-int(i))

			select {
			case <-exit:
				fmt.Printf(format, status.Downloaded, task.Size, h, 0, "[FINISH]")
				fmt.Println("\ndownload finished")
				wg.Done()
			default:
				if !pause {
					time.Sleep(time.Second * 1)
					fmt.Printf(format, status.Downloaded, task.Size, h, status.Speed, "[DOWNLOADING]")
					os.Stdout.Sync()
				} else {
					fmt.Printf(format, status.Downloaded, task.Size, h, 0, "[PAUSE]")
					os.Stdout.Sync()
					<-resume
					pause = false
				}
			}
		}
	}).OnPause(func() {
		pause = true
	}).OnResume(func() {
		resume <- true
	}).OnFinish(func() {
		exit <- true
	}).OnError(func(errcode int, err error) {
		log.Println(errcode, err)
	})


	// 开始下载
	task.Start()
	wg.Wait()
}


