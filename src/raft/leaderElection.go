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
	args := RequestVoteArgs{
		Term:         term,
		CandidateId:  rf.me,
		LastLogIndex: 0,
		LastLogTerm:  0,
	}

	var becomeLeader sync.Once
	for Id, _ := range rf.peers {
		if Id != rf.me {
			go rf.candidateRequest(Id, &votecount, &args, &becomeLeader)
		}
	}
	//当代码到这里的时候，已经跳出此函数了
}