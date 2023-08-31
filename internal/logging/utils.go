package logging

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"strings"
)

type MyFormatter struct {
}

func (m MyFormatter) Format(entry *log.Entry) ([]byte, error) {
	level := strings.ToUpper(entry.Level.String())
	if level == "WARNING" {
		level = "WARN"
	}

	const levelLen = 5
	if len(level) >= levelLen {
		level = level[:levelLen]
	} else {
		level = level + strings.Repeat(" ", levelLen-len(level))
	}

	filePos := ""
	if entry.HasCaller() && entry.Logger.Level >= log.DebugLevel {
		splits := strings.Split(entry.Caller.File, "/")
		fileName := splits[len(splits)-1]
		splits = strings.Split(entry.Caller.Function, ".")
		funcName := splits[len(splits)-1]
		filePos = fmt.Sprintf(" %s:%d(%s)", fileName, entry.Caller.Line, funcName)
	}

	s := fmt.Sprintf(
		"[%s %s%s] %s\n",
		entry.Time.Format("2006-01-02 15:04:05.000"), level,
		filePos,
		strings.TrimSuffix(entry.Message, "\n"),
	)
	return []byte(s), nil
}

var _ log.Formatter = &MyFormatter{}

func InitLog() {
	log.StandardLogger().ReportCaller = true
	log.SetFormatter(MyFormatter{})
}
