package mr

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"sync"
	"time"
)
import "log"
import "net/rpc"
import "hash/fnv"

//
// Map functions return a slice of KeyValue.
//
type KeyValue struct {
	Key   string
	Value string
}

type FileLk struct {
	tmpset []*os.File
	muset  []*sync.Mutex
}

//
// use ihash(key) % NReduce to choose the reduce
// task number for each KeyValue emitted by Map.
//
func ihash(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32() & 0x7fffffff)
}

//
// main/mrworker.go calls this function.
//
func Worker(mapf func(string, string) []KeyValue,
	reducef func(string, []string) string) {
	loop := 1
	negative := 0
	for {
	start:
		fmt.Printf("%dTH LOOP START!\n", loop)
		loop++
		wrk := WorkerParameter{}
		timef := TaskState{}
		IfSucc := call("Coordinator.TaskGet", &timef, &wrk)
		if !IfSucc {
			break
		}
		if len(wrk.File) == 0 {
			time.Sleep(10 * time.Second)
			fmt.Println("Waiting for input file")
			negative++
			if negative > 10 {
				break
			}
			goto start
		}

		//wrk.callmu = sync.Mutex{}
		negative = 0
		flk := FileLk{}
		wtMap := sync.WaitGroup{}
		flk.tmpset = make([]*os.File, wrk.Reduce)
		for i := 0; i < wrk.Reduce; i++ {
			tmpfile := "tmp" + strconv.Itoa(i) + ".txt"
			tmpfd, err := os.OpenFile(tmpfile, os.O_WRONLY|os.O_CREATE, 0660)
			if err != nil {
				panic("open file in Worker failed\n")
			}
			flk.tmpset[i] = tmpfd
			mu := sync.Mutex{}
			flk.muset = append(flk.muset, &mu)
		}

		for i := 0; i < len(wrk.File); i++ {
			InFd, err := os.Open(wrk.File[i])
			if err != nil {
				panic("Open input file failed\n")
			}
			wtMap.Add(1)
			go mapProc(&wrk, InFd, &wtMap, mapf, &flk, i)
		}
		wtMap.Wait()

		wtReduce := sync.WaitGroup{}
		for i := 0; i < wrk.Reduce; i++ {
			wtReduce.Add(1)
			go
		}
		time.Sleep(100 * time.Second)
	}
}

func mapProc(wrk *WorkerParameter, fd *os.File, wtMap *sync.WaitGroup, mapf func(string, string) []KeyValue, flk *FileLk, index int) {
	defer wtMap.Done()
	contents, err := ioutil.ReadAll(fd)
	if err != nil {
		panic("read file failed in mapPorc")
	}

	result := mapf("", string(contents))
	for _, elem := range result {
		hashindex := ihash(elem.Key) % wrk.Reduce
		flk.muset[hashindex].Lock()
		writer := bufio.NewWriter(flk.tmpset[hashindex])
		writer.WriteString(elem.Key + "\n")
		writer.Flush()
		flk.muset[hashindex].Unlock()
	}
	task := TaskState{done, 1, index}
	reply := Replyparameter{false}
	//wrk.callmu.Lock()
	call("Coordinator.MapFinished", &task, &reply)
	//wrk.callmu.Unlock()
}

func reduceProc(wrk *WorkerParameter, wtReduce *sync.WaitGroup, reducef func(string, string) []KeyValue, flk *FileLk) {

}

//
// send an RPC request to the coordinator, wait for the response.
// usually returns true.
// returns false if something goes wrong.
//
func call(rpcname string, args interface{}, reply interface{}) bool {
	// c, err := rpc.DialHTTP("tcp", "127.0.0.1"+":1234")
	sockname := coordinatorSock()
	c, err := rpc.DialHTTP("unix", sockname)
	if err != nil {
		log.Fatal("dialing:", err)
	}
	defer c.Close()

	err = c.Call(rpcname, args, reply)
	if err == nil {
		return true
	}

	fmt.Println(err)
	return false
}
