package mr

import (
	"fmt"
	"log"
	"sync"
	"time"
)
import "net"
import "os"
import "net/rpc"
import "net/http"

const (
	wait = iota
	busy
	done
	pending
)

type Coordinator struct {
	FileList []string
	NReduce  int

	activeTime time.Time
	timechan   chan bool
	timemu     sync.Mutex
	mapState   []int
	mapTime    []time.Time
	mapDone    bool

	reduceState []int
	reduceTime  []time.Time
	reduceDone  bool
}

func (c *Coordinator) keepAlive() {
	for {
		res := <-c.timechan
		if res {
			c.timemu.Lock()
			c.activeTime = time.Now().UTC()
			c.timemu.Unlock()
		} else {
			time.Sleep(time.Second)
		}
	}
}

// Your code here -- RPC handlers for the worker to call.

func (c *Coordinator) TaskGet(tss *TaskState, wrk *WorkerParameter) error {
	wrk.File = c.FileList
	wrk.Reduce = c.NReduce
	now := time.Now().UTC()
	wrk.TimeStart = now
	inum := len(c.FileList)
	for i := 0; i < inum; i++ {
		c.mapState[i] = tss.State
		c.mapTime[i] = now
	}
	c.timechan <- true
	return nil
}

func (c *Coordinator) MapFinished(task *TaskState, reply *Replyparameter) error {
	c.mapState[task.Index] = task.State
	c.mapTime[task.Index] = time.Now().UTC()
	fmt.Printf("Map Task %d Completed\n", task.Index)
	reply.Status = true
	c.timechan <- true
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
	c.timemu.Lock()
	dur := time.Since(c.activeTime) / time.Millisecond
	c.timemu.Unlock()
	fmt.Printf("pause time :%d\n", dur)
	if dur > 60*1000 {
		return true
	} else {
		return false
	}
}

//
// create a Coordinator.
// main/mrcoordinator.go calls this function.
// nReduce is the number of reduce tasks to use.
//
func MakeCoordinator(files []string, nReduce int) *Coordinator {
	c := Coordinator{}
	c.timechan = make(chan bool, 1000)
	c.timemu = sync.Mutex{}
	c.FileList = files
	inputNum := len(files)
	c.NReduce = nReduce
	go c.keepAlive()
	now := time.Now().UTC()
	c.timechan <- true
	c.mapState = make([]int, inputNum)
	c.mapTime = make([]time.Time, inputNum)
	c.mapDone = false
	for i := 0; i < inputNum; i++ {
		c.mapTime[i] = now
		c.mapState[i] = wait
	}
	c.reduceState = make([]int, c.NReduce)
	c.reduceTime = make([]time.Time, c.NReduce)
	c.reduceDone = false
	for i := 0; i < c.NReduce; i++ {
		c.reduceTime[i] = now
		c.reduceState[i] = wait
	}

	c.server()
	return &c
}
