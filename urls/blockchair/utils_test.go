package main

import (
	"fmt"
	"testing"
	"time"
)

func TestTimeParse(t *testing.T) {
	date := "20200325"
	tim, err := time.Parse("YYYYMMDD", date)
	if err != nil {
		t.Error(err)
	}
	fmt.Println(tim.Date())
}

func Test_timeToDate(t *testing.T) {
	// Time输出为20210521形式
	now := time.Now()
	date := timeToDate(now)
	t.Log(date, []byte(date))
	if date != "20210514" {
		t.Error("err")
	}
}

func Test_genUrls(t *testing.T) {
	// Time输出为20210521形式
	now := time.Now()
	prev := now.AddDate(0, 0, -2)
	next := now.AddDate(0, 0, 1)
	fmt.Println(prev)
	fmt.Println(now)
	fmt.Println(next)

	genUrls(prev, now)
}
