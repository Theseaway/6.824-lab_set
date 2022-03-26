package mr

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
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
	fmt.Println("function Worker called once")
	negative := 0
	for {
	start:
		wrk := WorkerParameter{}
		timef := TaskState{}
		IfSucc := call("Coordinator.TaskGet", &timef, &wrk)
		if !IfSucc {
			os.Exit(1)
		}
		if wrk.TaskType == 3 {
			os.Exit(1)
		}
		if len(wrk.File) == 0 {
			time.Sleep(5 * time.Second)
			negative++
			if negative > 1 {
				fmt.Println("No file input, program ends")
				os.Exit(1)
			}
			goto start
		}
		//wrk.callmu = sync.Mutex{}
		negative = 0
		wtMap := sync.WaitGroup{}
		wrk.tmpFilewr = sync.Mutex{}
		wrk.tmpFileList = map[int]bool{}
		for i := 0; i < wrk.Reduce; i++ {
			tlock := sync.Mutex{}
			wrk.tmpLockList = append(wrk.tmpLockList, tlock)
		}
		for i := 0; i < len(wrk.File); i++ {
			wtMap.Add(1)
			go mapProc(&wrk, &wtMap, mapf, i)
		}
		wtMap.Wait()

		reply := Replyparameter{}
		call("Coordinator.MapAllDone", &wrk, &reply)

		wtReduce := sync.WaitGroup{}
		for index, _ := range wrk.tmpFileList {
			wtReduce.Add(1)
			go reduceProc(&wrk, &wtReduce, index, reducef)
		}
		wtReduce.Wait()
	}
}

func mapProc(wrk *WorkerParameter, wtMap *sync.WaitGroup, mapf func(string, string) []KeyValue, index int) {
	defer wtMap.Done()

	fd, err := os.Open(wrk.File[index])
	if err != nil {
		panic(err)
	}
	contents, err := ioutil.ReadAll(fd)
	if err != nil {
		panic("read file failed in mapPorc")
	}
	fd.Close()

	flknmap := map[int]*os.File{}
	result := mapf(wrk.File[index], string(contents))
	for _, elem := range result {
		hashindex := ihash(elem.Key) % wrk.Reduce

		var fdnow *os.File
		if _, ok := flknmap[hashindex]; ok {
			fdnow = flknmap[hashindex]
		} else {
			// global lock for updating tmpFileList for next reduce step
			wrk.tmpFilewr.Lock()
			if _, ok := wrk.tmpFileList[hashindex]; !ok {
				wrk.tmpFileList[hashindex] = true
			}
			wrk.tmpFilewr.Unlock()
			// open file and ready to write
			tname := "tmp-" + strconv.Itoa(hashindex) + ".txt"
			fdnew, err := os.OpenFile(tname, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0660)
			if err != nil {
				panic(err)
			}
			fdnow = fdnew
			//update map
			flknmap[hashindex] = fdnow
		}
		wrk.tmpLockList[hashindex].Lock()
		writer := bufio.NewWriter(fdnow)
		writer.WriteString(elem.Key + " " + elem.Value + "\n")
		writer.Flush()
		wrk.tmpLockList[hashindex].Unlock()
	}

	for _, file := range flknmap {
		file.Close()
	}
	task := TaskState{done, 1, index}
	reply := Replyparameter{false}
	call("Coordinator.MapFinished", &task, &reply)
}

func reduceProc(wrk *WorkerParameter, wtReduce *sync.WaitGroup, index int, reducef func(string, []string) string) {
	defer wtReduce.Done()
	tmpname := "tmp-" + strconv.Itoa(index) + ".txt"
	fd, err := os.Open(tmpname)
	if err != nil {
		panic(err)
	}
	defer fd.Close()
	flk := FileLk{}
	flk.tmpset = make([]*os.File, wrk.Reduce)
	for i := 0; i < wrk.Reduce; i++ {
		tmpfile := "tmp-" + strconv.Itoa(i) + ".txt"
		tmpfd, err := os.OpenFile(tmpfile, os.O_WRONLY|os.O_CREATE, 0660)
		if err != nil {
			panic("open file in Worker failed\n")
		}
		flk.tmpset[i] = tmpfd
		mu := sync.Mutex{}
		flk.muset = append(flk.muset, &mu)
	}
	flk.muset[index].Lock()

	outname := "mr-out-" + strconv.Itoa(index) + ".txt"
	outfd, err := os.OpenFile(outname, os.O_WRONLY|os.O_CREATE, 0660)
	if err != nil {
		panic(err)
	}
	defer outfd.Close()
	outwriter := bufio.NewWriter(outfd)

	KeyValueMap := map[string][]string{}
	buf := bufio.NewReader(fd)
	for {
		line, err := buf.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			} else {
				fmt.Println("Read file error!", err)
				return
			}
		}
		lin := line[:len(line)-1]
		spres := strings.Split(lin, " ")
		if len(spres) < 1 {
			panic("split error, please check the word")
		}
		//fmt.Printf("length of spres is %d\n", len(spres))
		if len(spres) == 2 {
			if _, ok := KeyValueMap[spres[0]]; !ok {
				tmpslice := []string{}
				tmpslice = append(tmpslice, spres[1])
				KeyValueMap[spres[0]] = tmpslice
			} else {
				KeyValueMap[spres[0]] = append(KeyValueMap[spres[0]], spres[1])
			}
		}
	}
	for key_, value_ := range KeyValueMap {
		outwriter.WriteString(key_ + " " + reducef(key_, value_) + "\n")
		outwriter.Flush()
	}
	flk.muset[index].Unlock()

	task := TaskState{done, 2, index}
	reply := Replyparameter{false}
	call("Coordinator.MapFinished", &task, &reply)
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
