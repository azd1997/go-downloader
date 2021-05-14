package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/azd1997/blockchair_downloader/pool"
	"github.com/azd1997/blockchair_downloader/task"
)

const (
	urlFormat = "https://gz.blockchair.com/bitcoin/inputs/blockchair_bitcoin_inputs_%s.tsv.gz"
)

// 命令行格式：
// blockchair [-n 20] [20210315][-20210320]

var (
	nDownloaderFlag = flag.Int("n", 20, "指定使用最多n个下载器同时工作")
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
		start, end time.Time
		wg sync.WaitGroup
		)

	flag.Parse()
	if len(flag.Args()) != 1 && len(flag.Args()) != 0 {
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
	fmt.Printf("最大允许下载器数量：%d\n", numOfCD)

	// 解析时间
	if len(flag.Args()) == 1 {
		dateStr = flag.Arg(0)
	} else {	// ==0
		dateStr = timeToDate(time.Now())	// 下载今天的
	}
	dateStrSlice = strings.Split(dateStr, "-")
	if len(dateStrSlice) == 1 {
		dateStart, dateEnd = dateStrSlice[0], dateStrSlice[0]
	} else if len(dateStrSlice) == 2 {
		dateStart, dateEnd = dateStrSlice[0], dateStrSlice[1]
	} else {
		goto ERR
	}
	fmt.Printf("下载日期范围：%s-%s\n", dateStart, dateEnd)

	// 检查dateStart, dateEnd格式
	start, end, err = checkDateRange(dateStart, dateEnd)
	if err != nil {
		log.Fatalln(err)
	}

	// 生成Url列表
	urls = genUrls(start, end)

	// 根据url列表构建Task，并下载
	wg.Add(len(urls))
	tasks = make([]*task.Task, len(urls))
	for i=0; i<len(urls); i++ {
		go func(i int) {
			tasks[i], err = task.NewTask(urls[i])
			if err != nil {
				log.Fatalln(err)
			}
			err = tasks[i].Start()
			if err != nil {
				log.Fatalln(err)
			}
			wg.Done()
		}(i)
	}
	//tasks = tasks

	wg.Wait()

	fmt.Println("下载完成！！！")
	return

ERR:
	fmt.Println("确保命令行格式为：blockchair [-n 20] 20210315[-20210320]")
	os.Exit(-1)
}

func checkDateRange(dateStart, dateEnd string) (start, end time.Time, err error) {
	start, err = parseDate(dateStart)
	if err != nil {
		return
	}
	end, err = parseDate(dateEnd)
	if err != nil {
		return
	}
	if start.After(end) {
		err = errors.New("start > end")
		return
	}

	return
}

func parseDate(dateStr string) (date time.Time, err error) {
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
	year, err := strconv.Atoi(yearStr)
	if err != nil {
		return
	}
	month, err := strconv.Atoi(monthStr)
	if err != nil {
		return
	}
	day, err := strconv.Atoi(dayStr)
	if err != nil {
		return
	}
	// 日期不能晚于今天
	curYear, curMonth, curDay := time.Now().Date()
	if year<0 || year > curYear {
		err = errors.New("year<0 || year > curYear")
		return
	}
	if month < 1 || month > 12 ||
		(year == curYear && month > int(curMonth)) {
		err = errors.New("month < 1 || month > 12")
		return
	}
	if day < 1 || day > 31 ||
		(year == curYear && month == int(curMonth) && day > curDay){	// 粗略的限制
		err = errors.New("day < 1 || day > 31")
		return
	}
	// 构建成时间
	date = time.Date(year, time.Month(month), day, 0, 0, 0,0, time.Local)

	return
}

// Time输出为20210521形式
func timeToDate(t time.Time) string {
	str := fmt.Sprintf("%4d%2d%2d", t.Year(), t.Month(), t.Day())
	slice := []byte(str)
	for i:=0;i<len(str);i++ {
		if slice[i] == ' ' {
			slice[i] = '0'
		}
	}
	return string(slice)
}

func genUrls(start, end time.Time) []string {
	fmt.Println("待下载Url列表：")
	urls := make([]string, 0)
	for d:=start; !d.After(end); d=d.AddDate(0,0,1) {
		date := timeToDate(d)
		url := fmt.Sprintf(urlFormat, date)
		urls = append(urls, url)
		fmt.Println(url)
	}
	return urls
}

func genUrls2(start, end time.Time) []string {
	fmt.Println("待下载Url列表：")
	urls := make([]string, 0)
	for d:=start; !d.After(end); d.AddDate(0,0,1) {
		fmt.Println(start)
		fmt.Println(end)
		fmt.Println(d)

		date := timeToDate(d)
		url := fmt.Sprintf(urlFormat, date)
		urls = append(urls, url)
		fmt.Println(url)

		time.Sleep(time.Second)
	}
	return urls
}