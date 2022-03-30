package mr

import (
	"io"
	"log"
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
	MAPTASK    = 1
	REDUCETASK = 2
	TASKDONE   = 3
)

type TaskBase struct {
	File  string
	State int
	Time  time.Time
}

type Map struct {
	TaskBase
}

type Reduce struct {
	TaskBase
}

type Workers struct {
	MapWait     []Map
	MapDoing    []Map
	mLock       sync.Mutex
	ReduceWait  []Reduce
	ReduceDoing []Reduce
	rLock       sync.Mutex
	TmpList     TmpWait
	tmpFilelock sync.Mutex
}

type Coordinator struct {
	NReduce    int
	InputNum   int
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

func (c *Coordinator) TaskGet(tss *TaskReply, wrk *WorkerParameter) error {
	logFile, err := os.OpenFile("../../Log/ServerLog.txt", os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
	if err != nil {
		panic(err)
	}
	defer logFile.Close()
	mw := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(mw)

	c.wrk.mLock.Lock()
	if len(c.wrk.MapWait) > 0 {
		wrk.File = c.wrk.MapWait[len(c.wrk.MapWait)-1].File
		wrk.TaskType = MAPTASK
		wrk.Reduce = c.NReduce
		wrk.TimeStart = time.Now().UTC()
		wrk.TaskNum = len(c.wrk.MapWait) - 1
		log.Printf("map task out:\n%v\n", wrk)
		c.timechan <- true
		c.wrk.MapWait = c.wrk.MapWait[:len(c.wrk.MapWait)-1]
		c.wrk.mLock.Unlock()
		return nil
	}
	c.wrk.mLock.Unlock()

	c.wrk.rLock.Lock()
	if len(c.wrk.ReduceWait) > 0 {
		wrk.File = c.wrk.ReduceWait[len(c.wrk.ReduceWait)-1].File
		wrk.TaskType = REDUCETASK
		wrk.Reduce = c.NReduce
		wrk.TimeStart = time.Now().UTC()
		wrk.TaskNum = len(c.wrk.ReduceWait) - 1
		log.Printf("reduce task out:\n%v\n", wrk)
		c.timechan <- true
		c.wrk.ReduceWait = c.wrk.ReduceWait[:len(c.wrk.ReduceWait)-1]
		c.wrk.rLock.Unlock()
		return nil
	}
	c.wrk.rLock.Unlock()
	wrk.TaskType = 3
	return nil
}

func (c *Coordinator) TaskFinished(task *TaskReply, reply *Replyparameter) error {
	logFile, err := os.OpenFile("../../Log/ServerLog.txt", os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
	if err != nil {
		panic(err)
	}
	defer logFile.Close()
	mw := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(mw)

	if task.Tasktype == MAPTASK {
		c.wrk.mLock.Lock()
		mm := Map{
			TaskBase{File: task.File,
				State: done,
				Time:  time.Now().UTC()},
		}
		log.Printf("MapList task Finished and move to MapDoing List:\n%v\n", mm)
		c.wrk.MapDoing = append(c.wrk.MapDoing, mm)
		c.wrk.mLock.Unlock()
	} else if task.Tasktype == REDUCETASK {
		c.wrk.rLock.Lock()
		rr := Reduce{
			TaskBase{File: task.File,
				State: done,
				Time:  time.Now().UTC()},
		}
		log.Printf("Reduce task Finished and move to ReduceDoing List:\n%v\n", rr)
		c.wrk.ReduceDoing = append(c.wrk.ReduceDoing, rr)
		c.wrk.rLock.Unlock()
	}
	c.timechan <- true
	return nil
}

func (c *Coordinator) TmpFileAdd(FileList *TmpWait, reply *Replyparameter) error {
	logFile, err := os.OpenFile("../../Log/ServerLog.txt", os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
	if err != nil {
		panic(err)
	}
	defer logFile.Close()
	mw := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(mw)
	///////////////////////////////////////////////////////////////
	c.wrk.tmpFilelock.Lock()

	c.wrk.TmpList.File = append(c.wrk.TmpList.File, FileList.File...)

	c.wrk.tmpFilelock.Unlock()
	reply.Status = true
	log.Printf("tmpFile now:\n%v\n", c.wrk.TmpList.File)
	c.timechan <- true
	return nil
}

func (c *Coordinator) mapDone() bool {
	logFile, err := os.OpenFile("../../Log/ServerLog.txt", os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
	if err != nil {
		panic(err)
	}
	defer logFile.Close()
	mw := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(mw)
	res := false
	c.wrk.mLock.Lock()
	log.Println("$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$")
	log.Println("Checking Map task")
	if len(c.wrk.MapWait) == 0 || len(c.wrk.MapDoing) == 8 {
		log.Println("All map tasks done")
		res = true
	}
	c.wrk.mLock.Unlock()
	log.Println("$$$$$$$$$$$$$$$$$$$$$$$$$$$$$")
	c.timechan <- true
	return res
}

func (c *Coordinator) check() {
	logFile, err := os.OpenFile("../../Log/ServerLog.txt", os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
	if err != nil {
		panic(err)
	}
	defer logFile.Close()
	mw := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(mw)

	loop := 1
	for {
		c.timemu.Lock()
		log.Printf("launch %d check process\n", loop)
		c.timemu.Unlock()
		if c.mapDone() {
			log.Printf("number of tmp file list:%d\n", len(c.wrk.TmpList.File))
			c.wrk.rLock.Lock()
			for _, file := range c.wrk.TmpList.File {
				task := Reduce{}
				task.File = file
				task.State = wait
				task.Time = time.Now().UTC()
				c.wrk.ReduceWait = append(c.wrk.ReduceWait, task)
				log.Printf("tasks added to List:%v\n", task)
			}
			c.wrk.rLock.Unlock()
			c.wrk.TmpList.File = []string{}
			log.Println("All task move to the reduce list")
			break
		}
		time.Sleep(1000 * time.Millisecond)
		c.timemu.Lock()
		loop++
		c.timemu.Unlock()
		c.timechan <- true
	}
	log.Println("Check Process Over")
	c.timechan <- true
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
	if dur > 100*1000 {
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
	logFile, err := os.OpenFile("../../Log/ServerLog.txt", os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		panic(err)
	}
	defer logFile.Close()
	mw := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(mw)
	//fmt.Println("coordinator created\n")
	now := time.Now().UTC()
	c := Coordinator{}
	c.timechan = make(chan bool, 1000)
	c.timemu = sync.Mutex{}
	c.NReduce = nReduce
	c.wrk = Workers{}
	c.InputNum = len(files)
	for i := 0; i < c.InputNum; i++ {
		task := Map{}
		task.File = files[i]
		task.State = wait
		task.Time = now
		c.wrk.MapWait = append(c.wrk.MapWait, task)
	}
	go c.keepAlive()
	c.timechan <- true
	c.server()
	go c.check()
	log.Printf("Coordinator initialization complete\ncoordinator now:\nNReduce:%v \nInputNum:%v \n", c.NReduce, c.InputNum)
	for index, task := range c.wrk.MapWait {
		log.Printf("\ntask %d:\nFile:%v\nState:%v\nTime:%v\n", index, task.File, task.State, task.Time)
	}
	log.Println("#################################################")
	return &c
}
