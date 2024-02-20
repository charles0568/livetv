package service

import (
	"time"

	"github.com/zjyl1994/livetv/syncx"
)

type StatusInfo struct {
	Time   time.Time
	Status int
	Msg    string
}

const (
	Unknown = iota
	Ok
	Warning
	Error
	Expired
)

var statusCache syncx.Map[any, *StatusInfo]

func UpdateStatus(url any, status int, msg string) {
	if c, ok := statusCache.Load(url); ok {
		c.Msg = msg
		c.Status = status
		c.Time = time.Now()
	} else {
		statusCache.Store(url, &StatusInfo{
			Msg:    msg,
			Status: status,
			Time:   time.Now(),
		})
	}
}

func GetStatus(url any) StatusInfo {
	if c, ok := statusCache.Load(url); ok {
		return *c
	} else {
		return StatusInfo{
			Status: Unknown,
			Msg:    "Not yet parsed",
			Time:   time.Now(),
		}
	}
}

func DeleteStatus(url any) {
	statusCache.Delete(url)
}
