package raft

type Entry struct {
	Command interface{}
	Term    int
	Index   int
}

type AppendEntriesArgs struct {
	Term         int
	LeaderId     int
	PrevLogIndex int
	PrevLogTerm  int
	Entries      []Entry
	LeaderCommit int
}

type AppendEntriesReply struct {
	Term     int
	Success  bool
	Conflict bool
	XTerm    int
	XIndex   int
	XLen     int
}

func (rf *Raft) appendEntries(HeartBeat bool) {
	for peer, _ := range rf.peers {
		if peer == rf.me {
			rf.resetElectionTimer()
			continue
		}
		args := AppendEntriesArgs{
			Term:         rf.currentTerm,
			LeaderId:     rf.me,
			PrevLogIndex: 0,
			PrevLogTerm:  0,
			Entries:      []Entry{},
			LeaderCommit: rf.me,
		}
		go rf.leaderSendEntries(peer, &args)
	}
}

func (rf *Raft) leaderSendEntries(peer int, args *AppendEntriesArgs) {
	reply := AppendEntriesReply{}
	if rf.state == Leader {
		ok := rf.sendAppendEntries(peer, args, &reply)
		if !ok {
			return
		}
		if reply.Term > rf.currentTerm {
			rf.setNewTerm(reply.Term)
			return
		}
	}
}

func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	reply.Term = rf.currentTerm
	reply.Success = false
	if args.Term > rf.currentTerm {
		reply.Success = true
		reply.Term = args.Term
		rf.setNewTerm(args.Term)
		DPrintf("[%v]: 目前Term: %v, follower %v 收到 leader %v 的 AE 消息", rf.me, rf.currentTerm, rf.me, args.LeaderId)
		return
	}
	if args.Term < rf.currentTerm {
		DPrintf("[%v]: 目前Term: %v, 拒绝收到 old Leader %v 的 AE 消息，其Term为： %v", rf.me, rf.currentTerm, args.LeaderId, args.Term)
		reply.Success = false
		reply.Term = rf.currentTerm
		return
	}
	DPrintf("[%v]: 目前Term: %v, follower %v 收到 leader %v 的 AE 消息", rf.me, rf.currentTerm, rf.me, args.LeaderId)
	rf.resetElectionTimer()
	if rf.state == Candidate {
		rf.state = Follower
	}
	reply.Success = true
	return
}

func (rf *Raft) sendAppendEntries(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
	ok := rf.peers[server].Call("Raft.AppendEntries", args, reply)
	return ok
}
