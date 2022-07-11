package raft

import (
	"io"
	"log"
	"os"
)

var logger *log.Logger

func init() {
	writerCmd := os.Stdout
	writerFile, err := os.OpenFile("log.txt", os.O_WRONLY|os.O_CREATE, 0755)
	if err != nil {
		log.Fatalf("create file log.txt failed: %v", err)
	}
	logger = log.New(io.MultiWriter(writerCmd, writerFile), "", log.Lshortfile|log.LstdFlags)
}

// Debugging
const Debug = true

func DPrintf(format string, a ...interface{}) (n int, err error) {
	if Debug {
		logger.Printf(format, a...)
	}
	return
}

func DPrintVerbose(format string, a ...interface{}) (n int, err error) {
	if Debug {
		logger.Printf(format, a...)
	}
	return
}
