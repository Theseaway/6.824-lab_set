package raft

import "sync"

func (rf *Raft) leaderElection() {
	rf.currentTerm++
	DPrintf("[%v]: 目前Term: %v，服务器 %v 请求投票", rf.me, rf.currentTerm, rf.me)
	rf.state = Candidate
	rf.votedFor = rf.me
	rf.resetElectionTimer()
	term := rf.currentTerm
	votecount := 1
	servercount := 1
	log := rf.log.LastLog()
	args := RequestVoteArgs{
		Term:         term,
		CandidateId:  rf.me,
		LastLogIndex: log.Index,
		LastLogTerm:  log.Term,
	}
	DPrintf("[%d]: Last Log is --> %v", rf.me, args)
	var becomeLeader sync.Once
	for Id, _ := range rf.peers {
		if Id != rf.me {
			go rf.candidateRequest(Id, &votecount, &args, &becomeLeader, &servercount)
		}
	}
	//当代码到这里的时候，已经跳出此函数了
}
