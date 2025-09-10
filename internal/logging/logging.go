package logging

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

type Logger struct {
	json bool
	mu   sync.Mutex
}

func New(jsonMode bool) *Logger { return &Logger{json: jsonMode} }

func (l *Logger) log(level, msg string, kv map[string]interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if kv == nil { kv = map[string]interface{}{} }
	if l.json {
		kv["level"] = level
		kv["msg"] = msg
		kv["ts"] = time.Now().Format(time.RFC3339Nano)
		enc := json.NewEncoder(os.Stdout)
		_ = enc.Encode(kv)
		return
	}
	fmt.Printf("[%s] %s", level, msg)
	if len(kv) > 0 {
		fmt.Print(" ")
		for k, v := range kv {
			fmt.Printf("%s=%v ", k, v)
		}
	}
	fmt.Println()
}

func (l *Logger) Info(msg string, kv map[string]interface{})  { l.log("INFO", msg, kv) }
func (l *Logger) Warn(msg string, kv map[string]interface{})  { l.log("WARN", msg, kv) }
func (l *Logger) Error(msg string, kv map[string]interface{}) { l.log("ERROR", msg, kv) }
func (l *Logger) Debug(msg string, kv map[string]interface{}) { l.log("DEBUG", msg, kv) }
