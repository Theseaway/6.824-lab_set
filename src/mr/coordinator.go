package mr

import (
	"log"
	"strconv"
	"sync"
	"time"
)
import "net"
import "os"
import "net/rpc"
import "net/http"

const (
	none = iota
	wait
	busy
	done
	pending
)

const (
	mapTask    = 1
	reduceTask = 2
)

type fileAdd struct {
	name  string
	index int
}

type Map struct {
	State int
	Time  time.Time
}

type Reduce struct {
	State int
	Time  time.Time
}

type Workers struct {
	MapList    []Map
	mapmu      sync.Mutex
	ReduceList []Reduce
	reducemu   sync.Mutex
}

type Coordinator struct {
	MapFile    []string
	reduceFile []string
	NReduce    int
	wrk        Workers
	activeTime time.Time
	timechan   chan bool
	timemu     sync.Mutex
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
	// check the state of task
	for {
		WaitList := []fileAdd{}
		c.wrk.mapmu.Lock()
		for i := 0; i < len(c.MapFile); i++ {
			if c.wrk.MapList[i].State == wait || c.wrk.MapList[i].State == pending {
				tpMap := fileAdd{c.MapFile[i], i}
				WaitList = append(WaitList, tpMap)
			}
		}
		c.wrk.mapmu.Unlock()
		if len(WaitList) > 0 {
			c.wrk.mapmu.Lock()
			wrk.TaskType = mapTask
			wrk.Reduce = c.NReduce
			wrk.TimeStart = time.Now().UTC()
			for _, elem := range WaitList {
				wrk.File = append(wrk.File, elem.name)
				c.wrk.MapList[elem.index].State = busy
				c.wrk.MapList[elem.index].Time = time.Now().UTC()
			}
			c.timechan <- true
			c.wrk.mapmu.Unlock()
			return nil
		} else {
			c.wrk.reducemu.Lock()
			for i := 0; i < c.NReduce; i++ {
				if c.wrk.ReduceList[i].State == wait || c.wrk.ReduceList[i].State == pending {
					tpMap := fileAdd{c.reduceFile[i], i}
					WaitList = append(WaitList, tpMap)
				}
			}
			c.wrk.reducemu.Unlock()
			if len(WaitList) > 0 {
				c.wrk.reducemu.Lock()
				wrk.TaskType = reduceTask
				wrk.Reduce = c.NReduce
				wrk.TimeStart = time.Now().UTC()
				for i := 0; i < len(WaitList); i++ {
					wrk.File = append(wrk.File, WaitList[i].name)
					wrk.TaskNum = append(wrk.TaskNum, WaitList[i].index)
					c.wrk.ReduceList[WaitList[i].index].State = busy
					c.wrk.ReduceList[WaitList[i].index].Time = time.Now().UTC()
				}
				c.timechan <- true
				c.wrk.reducemu.Unlock()
				return nil
			} else {
				wrk.TaskType = 3
				return nil
			}
		}
	}
}

func (c *Coordinator) MapFinished(task *TaskState, reply *Replyparameter) error {
	if task.Tasktype == mapTask {
		c.wrk.mapmu.Lock()
		c.wrk.MapList[task.Index].State = task.State
		c.wrk.MapList[task.Index].Time = time.Now().UTC()
		c.wrk.mapmu.Unlock()
	} else if task.Tasktype == reduceTask {
		c.wrk.reducemu.Lock()
		c.wrk.ReduceList[task.Index].State = task.State
		c.wrk.ReduceList[task.Index].Time = time.Now().UTC()
		c.wrk.reducemu.Unlock()
	}
	c.timechan <- true
	return nil
}

func (c *Coordinator) MapAllDone(wrk *WorkerParameter, reply *Replyparameter) error {
	c.wrk.reducemu.Lock()
	for index, _ := range wrk.tmpFileList {
		c.wrk.ReduceList[index].State = wait
		c.wrk.ReduceList[index].Time = time.Now().UTC()
	}
	c.wrk.reducemu.Unlock()
	reply.Status = true
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
	//fmt.Printf("pause time :%d\n", dur)
	if dur > 10*1000 {
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
	//fmt.Println("coordinator created\n")
	c := Coordinator{}
	c.timechan = make(chan bool, 1000)
	c.timemu = sync.Mutex{}
	c.MapFile = files
	inputNum := len(files)
	c.NReduce = nReduce
	go c.keepAlive()
	now := time.Now().UTC()
	c.timechan <- true

	c.wrk = Workers{}
	c.wrk.mapmu = sync.Mutex{}
	c.wrk.MapList = make([]Map, inputNum)
	for i := 0; i < inputNum; i++ {
		c.wrk.MapList[i].Time = now
		c.wrk.MapList[i].State = wait
	}
	c.wrk.reducemu = sync.Mutex{}
	c.wrk.ReduceList = make([]Reduce, c.NReduce)
	for i := 0; i < c.NReduce; i++ {
		c.reduceFile = append(c.reduceFile, "tmp-"+strconv.Itoa(i)+".txt")
		c.wrk.ReduceList[i].Time = now
		c.wrk.ReduceList[i].State = none
	}
	c.server()
	return &c
}
