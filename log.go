package turnpike

import (
	glog "log"
	"os"
)

var (
	logFlags = glog.Ldate | glog.Ltime | glog.Lshortfile
	log      Logger
)

// Logger is an interface compatible with log.Logger.
type Logger interface {
	Println(v ...interface{})
	Printf(format string, v ...interface{})
}

type noopLogger struct{}

func (n noopLogger) Println(v ...interface{})               {}
func (n noopLogger) Printf(format string, v ...interface{}) {}

// setup logger for package, noop by default
func init() {
	if os.Getenv("DEBUG") != "" {
		log = glog.New(os.Stderr, "", logFlags)
	} else {
		log = noopLogger{}
	}
}

// change log output to stderr
func Debug() {
	log = glog.New(os.Stderr, "", logFlags)
}

func SetLogger(l Logger) {
	log = l
}

func logErr(err error) error {
	if err == nil {
		return nil
	}
	if l, ok := log.(*glog.Logger); ok {
		l.Output(2, err.Error())
	} else {
		log.Println(err)
	}
	return err
}
