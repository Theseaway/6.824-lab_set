package mr

//
// RPC definitions.
//
// remember to capitalize all names.
//

import (
	"os"
	"time"
)
import "strconv"

//
// example to show how to declare the arguments
// and reply for an RPC.
//

type WorkerParameter struct {
	File      string
	Reduce    int
	TimeStart time.Time
	TimeEnd   time.Time
	TaskType  int
	TaskNum   int
}

type TaskReply struct {
	File     string
	Tasktype int
}

type TmpWait struct {
	File []string
}

type Replyparameter struct {
	Status bool
}

// Add your RPC definitions here.

// Cook up a unique-ish UNIX-domain socket name
// in /var/tmp, for the coordinator.
// Can't use the current directory since
// Athena AFS doesn't support UNIX-domain sockets.
func coordinatorSock() string {
	s := "/var/tmp/824-mr-"
	s += strconv.Itoa(os.Getuid())
	return s
}
