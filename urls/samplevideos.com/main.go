package main

import (
	"github.com/azd1997/blockchair_downloader/pool"
	"github.com/azd1997/blockchair_downloader/task"
)

func main() {
	err := pool.Init(10)
	if err != nil {
		panic(err)
	}
	pool.Start()
	defer pool.Stop()

	url := "https://sample-videos.com/video123/mp4/720/big_buck_bunny_720p_1mb.mp4"
	t, err := task.NewTask(url)
	if err != nil {
		panic(err)
	}
	err = t.Start()
	if err != nil {
		panic(err)
	}
}
