package mr

import (
	"log"
	"time"
)
import "net"
import "os"
import "net/rpc"
import "net/http"

const (
	wait = iota
	busy
)

type Coordinator struct {
	fileList   []string
	ActiveTime time.Time
	nReduce    int

	mapState []int
	mapTime  []time.Time
	mapDone  bool

	reduceState []int
	reduceTime  []time.Time
	reduceDone  bool
}

// Your code here -- RPC handlers for the worker to call.

//
// an example RPC handler.
//
// the RPC argument and reply types are defined in rpc.go.
//
func (c *Coordinator) TaskGet(wrk *worker, timef *timeF) error {
	wrk.FileList = c.fileList
	wrk.NReduce = c.nReduce
	now := time.Now().UTC()
	c.ActiveTime = now
	timef.timeNow = now
	for i := 0; i < wrk.NReduce; i++ {
		c.mapState[i] = busy
		c.mapTime[i] = now
	}
	return nil
}

//
// start a thread that listens for RPCs from worker.go
//
func (c *Coordinator) server() {
	rpc.Register(c)
	rpc.HandleHTTP()

	sockname := coordinatorSock()
	os.Remove(sockname)
	l, e := net.Listen("unix", sockname)
	if e != nil {
		log.Fatal("listen error:", e)
	}
	go http.Serve(l, nil)
}

//
// main/mrcoordinator.go calls Done() periodically to find out
// if the entire job has finished.
//
func (c *Coordinator) Done() bool {
	ret := false

	// Your code here.

	return ret
}

//
// create a Coordinator.
// main/mrcoordinator.go calls this function.
// nReduce is the number of reduce tasks to use.
//
func MakeCoordinator(files []string, nReduce int) *Coordinator {
	c := Coordinator{}
	c.fileList = files
	inputNum := len(files)
	c.nReduce = nReduce
	now := time.Now().UTC()
	c.ActiveTime = now
	c.mapState = make([]int, inputNum)
	c.mapTime = make([]time.Time, inputNum)
	c.mapDone = false
	for i := 0; i < inputNum; i++ {
		c.mapTime[i] = now
		c.mapState[i] = wait
	}

	c.reduceState = make([]int, c.nReduce)
	c.reduceTime = make([]time.Time, c.nReduce)
	c.reduceDone = false
	for i := 0; i < c.nReduce; i++ {
		c.reduceTime[i] = now
		c.reduceState[i] = wait
	}

	c.server()
	return &c
}
