package debug

import "time"

type SqlStatement struct {
	Time      string        `json:"time,omitempty"`
	Operation string        `json:"operation,omitempty"`
	Sql       string        `json:"sql,omitempty"`
	Error     string        `json:"error,omitempty"`
	Params    []interface{} `json:"params,omitempty"`
	Duration  time.Duration `json:"duration,omitempty"`
}
