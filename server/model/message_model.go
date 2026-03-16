package model

import "time"

type Message struct {
	FromUserID  int
	ToUserId    int
	TextContent string
	Time        time.Time
}
