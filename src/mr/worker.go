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

	logfilename := "../../Log/W" + strconv.Itoa(time.Now().Day()) + ".txt"
	logFile, err := os.OpenFile(logfilename, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
	if err != nil {
		panic(err)
	}
	defer logFile.Close()
	mw := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(mw)

	for {
		wrk := WorkerParameter{}
		timef := TaskReply{}
		IfSucc := call("Coordinator.TaskGet", &timef, &wrk)
		log.Println("Server service TaskGet called")
		if !IfSucc {
			os.Exit(1)
		}
		if wrk.TaskType == TASKDONE {
			os.Exit(1)
		}
		if wrk.TaskType == MAPTASK {
			log.Println("MAPTASK GET")
			go mapProc(wrk.File, mapf, wrk.TaskNum, wrk.Reduce)
		} else if wrk.TaskType == REDUCETASK {
			go reduceProc(wrk.File, reducef)
		} else if wrk.TaskType == TASKDONE {
			os.Exit(1)
		} else {
			panic("wrong task type")
		}
	}
}

func mapProc(file string, mapf func(string, string) []KeyValue, taskNum, Reduce int) {
	logfilename := "../../Log/W" + strconv.Itoa(time.Now().Day()) + ".txt"
	logFile, err := os.OpenFile(logfilename, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
	if err != nil {
		panic(err)
	}
	defer logFile.Close()
	mw := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(mw)

	log.Printf("File %s get", file)
	fd, err := os.Open(file)
	if err != nil {
		panic(err)
	}
	contents, err := ioutil.ReadAll(fd)
	if err != nil {
		panic("read file failed in mapPorc")
	}
	fd.Close()

	flknmap := map[int]*os.File{}
	FileAdd := []string{}
	for i := 0; i < Reduce; i++ {
		tname := "tmp-" + strconv.Itoa(taskNum) + "-" + strconv.Itoa(i) + ".txt"
		FileAdd = append(FileAdd, tname)
		fdnew, err := os.OpenFile(tname, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0660)
		if err != nil {
			panic(err)
		}
		flknmap[i] = fdnew
	}
	log.Printf("MAPTASK's tmp file %v created\n", FileAdd)
	result := mapf(file, string(contents))
	for _, elem := range result {
		hashindex := ihash(elem.Key) % Reduce
		writer := bufio.NewWriter(flknmap[hashindex])
		writer.WriteString(elem.Key + " " + elem.Value + "\n")
		writer.Flush()
	}
	for _, file := range flknmap {
		file.Close()
	}

	input := TmpWait{FileAdd}
	reply := Replyparameter{false}
	call("Coordinator.TmpFileAdd", &input, &reply)

	task := TaskReply{file, MAPTASK}
	reply = Replyparameter{false}
	call("Coordinator.TaskFinished", &task, &reply)
	log.Println("Server service TaskFinished called")

}

func reduceProc(file string, reducef func(string, []string) string) {
	logfilename := "../../Log/W" + strconv.Itoa(time.Now().Day()) + ".txt"
	logFile, err := os.OpenFile(logfilename, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
	if err != nil {
		panic(err)
	}
	defer logFile.Close()
	mw := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(mw)

	log.Printf("reduce task get :%v\n", file)
	fd, err := os.Open(file)
	if err != nil {
		panic(err)
	}
	defer fd.Close()
	kvMap := map[string][]string{}
	buf := bufio.NewReader(fd)
	for {
		lineraw, err := buf.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				//log.Println("File read ok!")
				break
			} else {
				log.Println("Read file error!", err)
				return
			}
		}
		line := strings.TrimSpace(lineraw)
		splitRes := strings.Fields(line)
		if _, ok := kvMap[splitRes[0]]; !ok {
			tmpslice := []string{}
			tmpslice = append(tmpslice, splitRes[1])
			kvMap[splitRes[0]] = tmpslice
		} else {
			kvMap[splitRes[0]] = append(kvMap[splitRes[0]], splitRes[1])
		}
	}
	outname := "mr-out-" + file
	outfd, err := os.OpenFile(outname, os.O_WRONLY|os.O_CREATE, 0660)
	if err != nil {
		panic(err)
	}
	defer outfd.Close()
	log.Printf("reduce output created :%v\n", outname)
	writer := bufio.NewWriter(outfd)
	for key, val := range kvMap {
		result := reducef(key, val)
		writer.WriteString(key + " " + result + "\n")
	}
	writer.Flush()
	task := TaskReply{file, REDUCETASK}
	reply := Replyparameter{false}
	call("Coordinator.TaskFinished", &task, &reply)
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
