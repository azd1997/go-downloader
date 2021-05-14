package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/azd1997/blockchair_downloader/pool"
	"github.com/azd1997/blockchair_downloader/task"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

// 命令行格式：
// blockchair [-n 20] 20210315[-20210320]

var (
	nDownloaderFlag = flag.Int("n", 100, "指定使用最多n个下载器同时工作")
)

func main() {

	var (
		numOfCD int
		dateStr string
		dateStrSlice []string
		dateStart, dateEnd string
		urls []string
		err error
		i int
		tasks []*task.Task
	)

	flag.Parse()
	if len(flag.Args()) != 1 {
		goto ERR
	}

	// 初始化下载器池
	numOfCD = *nDownloaderFlag
	err = pool.Init(numOfCD)
	if err != nil {
		log.Fatalln(err)
	}
	pool.Start()
	defer pool.Stop()

	// 解析时间
	dateStr = flag.Arg(0)
	dateStrSlice = strings.Split(dateStr, "-")
	if len(dateStrSlice) == 1 {
		dateStart, dateEnd = dateStrSlice[0], dateStrSlice[0]
	} else if len(dateStrSlice) == 2 {
		dateStart, dateEnd = dateStrSlice[0], dateStrSlice[1]
	} else {
		goto ERR
	}

	// 检查dateStart, dateEnd格式
	checkDateRange(dateStart, dateEnd)

	// 生成Url列表
	urls = []string{}

	// 根据url列表构建Task，并下载
	tasks = make([]*task.Task, len(urls))
	for i=0; i<len(urls); i++ {
		tasks[i], err = task.NewTask(urls[i])
		if err != nil {
			log.Fatalln(err)
		}
		err = tasks[i].Start()
		if err != nil {
			log.Fatalln(err)
		}
	}
	tasks = tasks

ERR:
	fmt.Println("确保命令行格式为：blockchair [-n 20] 20210315[-20210320]")
	os.Exit(-1)
}

func checkDateRange(dateStart, dateEnd string) error {
	timeStart, err := time.Parse("YYYYMMDD", dateStart)
	if err != nil {
		return err
	}
	fmt.Println(timeStart.Date())
	return nil
}

func parseDate(dateStr string) (year, month, day int, err error) {
	if len(dateStr) != 8 {
		err = errors.New("len(dateStr) != 8")
		return
	}
	// 必须都是数字
	for i:=0; i<8; i++ {
		if dateStr[i] > '9' || dateStr[i] < '0' {
			err = errors.New("dateStr[i] > '9' || dateStr[i] < '0'")
			return
		}
	}
	// 解析出年月日
	yearStr := dateStr[:4]
	monthStr := dateStr[4:6]
	dayStr := dateStr[6:]
	year, err = strconv.Atoi(yearStr)
	if err != nil {
		return
	}
	month, err = strconv.Atoi(monthStr)
	if err != nil {
		return
	}
	day, err = strconv.Atoi(dayStr)
	if err != nil {
		return
	}
	curYear, curMonth, curDay := time.Now().Month()
	if year<0 || year > curYear {
		err = errors.New("year<0 || year > curYear")
		return
	}
	if month < 1 || month > 12 ||
		(year == curYear && month > curMonth) {
		err = errors.New("month < 1 || month > 12")
		return
	}
	if day < 1 || day > 31 {	// 粗略的限制
		err = errors.New("day < 1 || day > 31")
		return
	}
	return
}
