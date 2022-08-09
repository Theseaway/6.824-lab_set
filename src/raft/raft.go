package raft

//
// this is an outline of the API that raft must expose to
// the service (or tester). see comments below for
// each of these functions for more details.
//
// rf = Make(...)
//   create a new Raft server.
// rf.Start(command interface{}) (index, term, isleader)
//   start agreement on a new log entry
// rf.GetState() (term, isLeader)
//   ask a Raft for its current term, and whether it thinks it is leader
// ApplyMsg
//   each time a new entry is committed to the log, each Raft peer
//   should send an ApplyMsg to the service (or tester)
//   in the same server.
//

import (
	"6.824/labgob"
	"bytes"
	"fmt"
	"math/rand"
	"strings"

	//	"bytes"
	"sync"
	"sync/atomic"
	"time"

	//	"6.824/labgob"
	"6.824/labrpc"
)

//
// as each Raft peer becomes aware that successive log entries are
// committed, the peer should send an ApplyMsg to the service (or
// tester) on the same server, via the applyCh passed to Make(). set
// CommandValid to true to indicate that the ApplyMsg contains a newly
// committed log entry.
//
// in part 2D you'll want to send other kinds of messages (e.g.,
// snapshots) on the applyCh, but set CommandValid to false for these
// other uses.
//

type RaftState string

const (
	Follower  RaftState = "Follower"
	Candidate           = "Candidate"
	Leader              = "Leader"
)

type ApplyMsg struct {
	CommandValid bool
	Command      interface{}
	CommandIndex int

	// For 2D:
	SnapshotValid bool
	Snapshot      []byte
	SnapshotTerm  int
	SnapshotIndex int
}

//
// A Go object implementing a single Raft peer.
//
type Raft struct {
	mu        sync.Mutex          // Lock to protect shared access to this peer's state
	peers     []*labrpc.ClientEnd // RPC end points of all peers
	persister *Persister          // Object to hold this peer's persisted state
	me        int                 // this peer's index into peers[]
	dead      int32               // set by Kill()

	// Your data here (2A, 2B, 2C).
	// Look at the paper's Figure 2 for a description of what
	// state a Raft server must maintain.
	LeaderID  int
	LastIndex int
	LastTerm  int

	state       RaftState
	currentTerm int
	votedFor    int
	commitIndex int
	lastApplied int
	nextIndex   []int
	matchIndex  []int
	log         Log
	//magic time
	electionTime time.Time
	heartBeat    time.Duration

	applyCh   chan ApplyMsg
	applyCond *sync.Cond
}

// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {
	var term int
	var isleader bool
	rf.mu.Lock()
	defer rf.mu.Unlock()
	term = rf.currentTerm
	isleader = rf.state == Leader
	DPrintf("[%v]: GET STATE; 目前Term: %v, 服务器状态： %v", rf.me, rf.currentTerm, rf.state)
	// Your code here (2A).
	return term, isleader
}

func (rf *Raft) persistData() []byte {
	w := new(bytes.Buffer)
	e := labgob.NewEncoder(w)
	e.Encode(rf.currentTerm)
	e.Encode(rf.votedFor)
	e.Encode(rf.log)
	data := w.Bytes()
	return data
}

//
// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
//
func (rf *Raft) persist() {
	// Your code here (2C).
	// Example:
	data := rf.persistData()
	rf.persister.SaveRaftState(data)
	// w := new(bytes.Buffer)
	// e := labgob.NewEncoder(w)
	// e.Encode(rf.xxx)
	// e.Encode(rf.yyy)
	// data := w.Bytes()
	// rf.persister.SaveRaftState(data)
}

//
// restore previously persisted state.
//
func (rf *Raft) readPersist(data []byte) {
	if data == nil || len(data) < 1 { // bootstrap without any state?
		return
	}
	r := bytes.NewBuffer(data)
	d := labgob.NewDecoder(r)
	var currentTerm int
	var votedFor int
	var logs Log
	if d.Decode(&currentTerm) != nil || d.Decode(&votedFor) != nil || d.Decode(&logs) != nil {
		panic("something wrong")
	} else {
		rf.currentTerm = currentTerm
		rf.votedFor = votedFor
		rf.log = logs
	}
	// Your code here (2C).
	// Example:
	// r := bytes.NewBuffer(data)
	// d := labgob.NewDecoder(r)
	// var xxx
	// var yyy
	// if d.Decode(&xxx) != nil ||
	//    d.Decode(&yyy) != nil {
	//   error...
	// } else {
	//   rf.xxx = xxx
	//   rf.yyy = yyy
	// }
}

//
// A service wants to switch to snapshot.  Only do so if Raft hasn't
// have more recent info since it communicate the snapshot on applyCh.
//
func (rf *Raft) CondInstallSnapshot(lastIncludedTerm int, lastIncludedIndex int, snapshot []byte) bool {

	// Your code here (2D).

	return true
}

// the service says it has created a snapshot that has
// all info up to and including index. this means the
// service no longer needs the log through (and including)
// that index. Raft should now trim its log as much as possible.
func (rf *Raft) Snapshot(index int, snapshot []byte) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	// Your code here (2D).
	if index >= rf.log.LastLog().Index || index < rf.log.Index0 {
		return
	}
	if index > rf.commitIndex {
		return
	}

	rf.LastIndex = index
	rf.LastTerm = rf.log.at(index).Term

	tmpLog := []Entry{}
	for i := index + 1; i <= rf.log.LastLog().Index; i++ {
		tmpLog = append(tmpLog, *rf.log.at(i))
	}
	rf.log.Entries = tmpLog
	rf.log.Index0 = index + 1
	rf.persist()
	rf.persister.SaveStateAndSnapshot(rf.persistData(), snapshot)
}

//
// the service using Raft (e.g. a k/v server) wants to start
// agreement on the next command to be appended to Raft's log. if this
// server isn't the leader, returns false. otherwise start the
// agreement and return immediately. there is no guarantee that this
// command will ever be committed to the Raft log, since the leader
// may fail or lose an election. even if the Raft instance has been killed,
// this function should return gracefully.
//
// the first return value is the index that the command will appear at
// if it's ever committed. the second return value is the current
// term. the third return value is true if this server believes it is
// the leader.
//
func (rf *Raft) Start(command interface{}) (int, int, bool) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	if rf.state != Leader {
		return -1, rf.currentTerm, false
	}
	log := rf.log.LastLog()
	entry := Entry{
		Command: command,
		Term:    rf.currentTerm,
		Index:   log.Index + 1,
	}
	DPrintf("[%d]: START ENTRY (cmd,term,index)--> %v", rf.me, entry)
	rf.log.append(entry)
	rf.persist()
	DPrintf("[%d]: Leader entry now is %v", rf.me, rf.log.Entries)
	rf.appendEntries(false)
	return entry.Index, entry.Term, true
}

//
// the tester doesn't halt goroutines created by Raft after each test,
// but it does call the Kill() method. your code can use killed() to
// check whether Kill() has been called. the use of atomic avoids the
// need for a lock.
//
// the issue is that long-running goroutines use memory and may chew
// up CPU time, perhaps causing later tests to fail and generating
// confusing debug output. any goroutine with a long-running loop
// should call killed() to check whether it should stop.
//
func (rf *Raft) Kill() {
	atomic.StoreInt32(&rf.dead, 1)
	// Your code here, if desired.
}

func (rf *Raft) killed() bool {
	z := atomic.LoadInt32(&rf.dead)
	return z == 1
}

// The ticker go routine starts a new election if this peer hasn't received
// heartsbeats recently.
func (rf *Raft) ticker() {
	for rf.killed() == false {
		// Your code here to check if a leader election should
		// be started and to randomize sleeping time using
		// time.Sleep().
		time.Sleep(rf.heartBeat)
		rf.mu.Lock()
		if rf.state == Leader {
			rf.appendEntries(true)
		}
		if time.Now().After(rf.electionTime) {
			rf.leaderElection()
		}
		rf.mu.Unlock()
	}
}

//
// the service or tester wants to create a Raft server. the ports
// of all the Raft servers (including this one) are in peers[]. this
// server's port is peers[me]. all the servers' peers[] arrays
// have the same order. persister is a place for this server to
// save its persistent state, and also initially holds the most
// recent saved state, if any. applyCh is a channel on which the
// tester or service expects Raft to send ApplyMsg messages.
// Make() must return quickly, so it should start goroutines
// for any long-running work.
//
func Make(peers []*labrpc.ClientEnd, me int,
	persister *Persister, applyCh chan ApplyMsg) *Raft {
	rf := &Raft{}
	rf.peers = peers
	rf.persister = persister
	rf.me = me
	rf.currentTerm = 0
	rf.votedFor = -1
	rf.mu = sync.Mutex{}

	rf.log = makeEmptyLog()
	rf.log.append(Entry{-1, 0, 0})
	rf.matchIndex = make([]int, len(rf.peers))
	rf.nextIndex = make([]int, len(rf.peers))
	rf.resetElectionTimer()
	rf.commitIndex = 0
	rf.lastApplied = 0
	rf.state = Follower
	rf.heartBeat = 50 * time.Millisecond

	rf.applyCh = applyCh
	rf.applyCond = sync.NewCond(&rf.mu)
	rand.Seed(int64(rf.me * 99))
	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())
	// start ticker goroutine to start elections
	go rf.ticker()
	go rf.LoopCommit()
	return rf
}

func (rf *Raft) SendMsg() {
	rf.applyCond.Broadcast()
	DPrintf("[%v]: rf.applyCond.Broadcast()", rf.me)
}

func (rf *Raft) CommitInfo() string {
	nums := []string{}
	for i := rf.log.Index0; i <= rf.log.LastLog().Index && i <= rf.lastApplied; i++ {
		nums = append(nums, fmt.Sprintf("%4d", rf.log.at(i).Command))
	}
	return fmt.Sprint(strings.Join(nums, "|"))
}

func (rf *Raft) LoopCommit() {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	//defer DPrintf("[%d]: function LoopCommit end", rf.me)
	for !rf.killed() {
		// all server rule 1
		//DPrintf("[%d]: function LoopCommit start", rf.me)
		if rf.commitIndex > rf.lastApplied && rf.log.LastLog().Index > rf.lastApplied {
			rf.lastApplied++
			//DPrintf("[%d]: commitIndex --> %d; logIndex --> %d; lastApplied --> %d", rf.me, rf.commitIndex, rf.log.LastLog().Index, rf.lastApplied)
			applyMsg := ApplyMsg{
				CommandValid: true,
				Command:      rf.log.at(rf.lastApplied).Command,
				CommandIndex: rf.lastApplied,
			}
			DPrintVerbose("[%v]: COMMIT $%d#: %v", rf.me, rf.lastApplied, rf.CommitInfo())
			rf.mu.Unlock()
			rf.applyCh <- applyMsg
			rf.mu.Lock()
		} else {
			//DPrintf("[%d]: function LoopCommit wait", rf.me)
			rf.applyCond.Wait()
			//DPrintf("[%d]: applyCond wake up", rf.me)
			//DPrintf("[%d]: commitIndex --> %d; logIndex --> %d; lastApplied --> %d", rf.me, rf.commitIndex, rf.log.LastLog().Index, rf.lastApplied)
		}
	}
}
