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
	LastLogIndex int
	LastLogTerm  int
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
	//DPrintf("[%d]: 等待时间为 --> %v", rf.me, electionTimeout)
	rf.electionTime = t.Add(electionTimeout)
}

func (rf *Raft) setNewTerm(term int) {
	if term > rf.currentTerm || rf.currentTerm == 0 {
		rf.state = Follower
		rf.currentTerm = term
		rf.votedFor = -1
		DPrintf("[%d]: 设置term为 --> %v\n", rf.me, rf.currentTerm)
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
		DPrintf("[%d]: 拒绝来自服务器 %d 的投票请求", rf.me, args.CandidateId)
		return
	}
	//本服务器term比候选者小，直接转变为追随者
	if args.Term > rf.currentTerm {
		rf.setNewTerm(args.Term)
	}

	lastLog := rf.log.LastLog()
	valid := args.LastLogTerm > lastLog.Term || (args.LastLogTerm == lastLog.Term && args.LastLogIndex >= lastLog.Index)
	//if (rf.votedFor == -1 || rf.votedFor == args.CandidateId) && valid {
	if (rf.votedFor == -1 || rf.votedFor == args.CandidateId) && valid {
		rf.votedFor = args.CandidateId
		reply.VoteGranted = true
		DPrintf("[%d]: 接收来自服务器 %d 的投票请求; Term --> %d", rf.me, args.CandidateId, rf.currentTerm)
		rf.resetElectionTimer()
		rf.persist()
	} else {
		reply.VoteGranted = false
		reply.Term = rf.currentTerm
	}
}

func (rf *Raft) sendRequestVote(Id int, args *RequestVoteArgs, reply *RequestVoteReply) bool {
	ok := rf.peers[Id].Call("Raft.RequestVote", args, reply)
	return ok
}

func (rf *Raft) candidateRequest(Id int, count *int, args *RequestVoteArgs, becomeLeader *sync.Once, server *int) {
	reply := RequestVoteReply{}
	ok := rf.sendRequestVote(Id, args, &reply)
	*server++
	if !ok {
		return
	}
	rf.mu.Lock()
	defer rf.mu.Unlock()
	if reply.VoteGranted == false && reply.Term > rf.currentTerm {
		DPrintf("[%v]: 有新的Term存在：%v", rf.me, reply.Term)
		rf.setNewTerm(reply.Term)
		return
	}
	if reply.VoteGranted {
		*count++
		DPrintf("[%v]： 收到投票，目前票数： %v", rf.me, *count)
	}
	//确保目前仍然是候选者
	if 2*(*count) > len(rf.peers) && rf.currentTerm == args.Term && rf.state == Candidate {
		becomeLeader.Do(func() {
			DPrintf("[%v]：投票过半，提前结束", rf.me)
			rf.state = Leader
			LastLog := rf.log.LastLog()
			for index, _ := range rf.peers {
				rf.nextIndex[index] = LastLog.Index + 1
				rf.matchIndex[index] = 0
			}
			rf.appendEntries(true)
		})
	} else if *server == len(rf.peers) && 2*(*count) < len(rf.peers) {
		rf.state = Follower
		rf.resetElectionTimer()
		DPrintf("[%d]: 没收到足够的选票，转为 follower", rf.me)
	}
}
