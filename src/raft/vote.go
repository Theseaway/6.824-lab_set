package raft

import (
	"math/rand"
	"sync"
	"time"
)

//
// example RequestVote RPC arguments structure.
// field names must start with capital letters!
//
type RequestVoteArgs struct {
	Term         int
	CandidateId  int
	LastLogIndex uint32
	LastLogTerm  uint32
}

//
// example RequestVote RPC reply structure.
// field names must start with capital letters!
//
type RequestVoteReply struct {
	Term        int
	VoteGranted bool
}

func (rf *Raft) resetElectionTimer() {
	t := time.Now()
	electionTimeout := time.Duration(150+rand.Intn(150)) * time.Millisecond
	rf.electionTime = t.Add(electionTimeout)
}

func (rf *Raft) setNewTerm(term int) {
	if term > rf.currentTerm || rf.currentTerm == 0 {
		rf.state = Follower
		rf.currentTerm = term
		rf.votedFor = -1
		DPrintf("[%d]: set term to %v\n", rf.me, rf.currentTerm)
		rf.persist()
		rf.resetElectionTimer()
	}
}

//
// example RequestVote RPC handler.
//
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	// 本服务器term比候选者大，拒绝
	if args.Term < rf.currentTerm {
		reply.Term = rf.currentTerm
		reply.VoteGranted = false
	}
	//本服务器term比候选者小，直接转变为追随者
	if args.Term > rf.currentTerm {
		rf.setNewTerm(args.Term)
	}

	if rf.votedFor == -1 || rf.votedFor == args.CandidateId {
		rf.votedFor = args.CandidateId
		reply.VoteGranted = true
		DPrintf("[%v]: 当前Term： %v, 给候选 %v 投票", rf.me, rf.currentTerm, rf.votedFor)
	} else {
		reply.Term = rf.currentTerm
		reply.VoteGranted = false
	}
}

//
// example code to send a RequestVote RPC to a server.
// server is the index of the target server in rf.peers[].
// expects RPC arguments in args.
// fills in *reply with RPC reply, so caller should
// pass &reply.
// the types of the args and reply passed to Call() must be
// the same as the types of the arguments declared in the
// handler function (including whether they are pointers).
//
// The labrpc package simulates a lossy network, in which servers
// may be unreachable, and in which requests and replies may be lost.
// Call() sends a request and waits for a reply. If a reply arrives
// within a timeout interval, Call() returns true; otherwise
// Call() returns false. Thus Call() may not return for a while.
// A false return can be caused by a dead server, a live server that
// can't be reached, a lost request, or a lost reply.
//
// Call() is guaranteed to return (perhaps after a delay) *except* if the
// handler function on the server side does not return.  Thus there
// is no need to implement your own timeouts around Call().
//
// look at the comments in ../labrpc/labrpc.go for more details.
//
// if you're having trouble getting RPC to work, check that you've
// capitalized all field names in structs passed over RPC, and
// that the caller passes the address of the reply struct with &, not
// the struct itself.
//
func (rf *Raft) sendRequestVote(Id int, args *RequestVoteArgs, reply *RequestVoteReply) bool {
	ok := rf.peers[Id].Call("Raft.RequestVote", args, reply)
	return ok
}

func (rf *Raft) candidateRequest(Id int, count *int, args *RequestVoteArgs, becomeLeader *sync.Once) {
	reply := RequestVoteReply{}
	ok := rf.sendRequestVote(Id, args, &reply)
	if !ok {
		return
	}
	rf.mu.Lock()
	defer rf.mu.Unlock()
	if reply.VoteGranted == false {
		if reply.Term > rf.currentTerm {
			DPrintf("[%v]: 有新的Term存在：%v", rf.me, reply.Term)
			rf.setNewTerm(reply.Term)
			return
		}
		return
	}
	*count++
	DPrintf("[%v]： 收到投票，目前票数： %v", rf.me, *count)
	if 2*(*count) > len(rf.peers) && rf.currentTerm == args.Term && rf.state == Candidate {
		becomeLeader.Do(func() {
			DPrintf("[%v]：投票过半，提前结束", rf.me)
			rf.state = Leader
			rf.appendEntries(true)
		})
	}

}
