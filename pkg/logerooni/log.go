package logerooni

import (
	log "log/slog"
	"os"
	"fmt"
)

func init(){
	log.SetDefault(log.New(log.NewTextHandler(os.Stdout, nil)))
}

// this is the way the go foundation wants me to do this.
// https://pkg.go.dev/log/slog#hdr-Wrapping_output_methods

func Debug(msg string){
	log.Debug(msg)
}

func Debugf(msg string, args ...any){
	Debug(fmt.Sprintf(msg, args...))
}

func Info(msg string){
	log.Info(msg)
}

func Infof(msg string, args ...any) {
	Info(fmt.Sprintf(msg, args...))
}

func Error(msg string){
	log.Error(msg)
}

func Errorf(msg string, args ...any){
	Error(fmt.Sprintf(msg, args...))
}

func Fatal(msg string) {
	log.Error(msg)
	os.Exit(1)
}

func Fatalf(msg string, args ...any) {
	Fatal(fmt.Sprintf(msg, args...))
}