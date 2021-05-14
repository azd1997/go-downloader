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
