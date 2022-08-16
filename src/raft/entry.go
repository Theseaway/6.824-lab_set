package raft

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

type SnapShotArgs struct {
	Term              int
	LeaderID          int
	LastIncludedIndex int
	LastIncludedTerm  int
	SnapShot          []byte
	Offset            int
	Done              bool
}

type SnapShotReply struct {
	Term int
}

func (rf *Raft) appendEntries(HeartBeat bool) {
	lastLog := rf.log.LastLog()
	for peer, _ := range rf.peers {
		if peer == rf.me {
			rf.resetElectionTimer()
			continue
		}
		// first time: next index start at lastlog.index + 1
		// after: from xterm and xindex
		if lastLog.Index >= rf.nextIndex[peer] || HeartBeat {
			nextIndex := rf.nextIndex[peer]
			// 完全不匹配, 从头给出
			if nextIndex <= 0 {
				nextIndex = 1
			}
			// 该follower的日志超出当前长度
			if lastLog.Index+1 < nextIndex {
				nextIndex = lastLog.Index
			}
			DPrintf("[%d]: state --> %v; nextIndex on server %d is %d", rf.me, rf.state, peer, rf.nextIndex[peer])
			// 获取可能匹配的prevLog的Index和Term，然后进行再次的匹配
			// 如果不匹配，会重新再进行匹配
			if nextIndex > rf.log.Index0 {
				prevLog := rf.log.at(nextIndex - 1)
				args := AppendEntriesArgs{
					Term:         rf.currentTerm,
					LeaderId:     rf.me,
					PrevLogIndex: prevLog.Index,
					PrevLogTerm:  prevLog.Term,
					//依据nextIndex发送所需要的日志
					Entries:      make([]Entry, lastLog.Index-nextIndex+1),
					LeaderCommit: rf.commitIndex,
				}
				copy(args.Entries, rf.log.Tail(nextIndex))
				go rf.leaderSendEntries(peer, &args)
			} else {
				go rf.SendSnapshot(peer)
			}
		}
	}
}

func (rf *Raft) SendSnapshot(peer int) {
	rf.mu.Lock()
	args := SnapShotArgs{
		Term:              rf.currentTerm,
		LeaderID:          rf.LeaderID,
		LastIncludedIndex: rf.LastIndex,
		LastIncludedTerm:  rf.LastTerm,
		SnapShot:          rf.persister.snapshot,
		Offset:            0,
		Done:              true,
	}
	rf.mu.Unlock()
	reply := SnapShotReply{}
	ok := rf.peers[peer].Call("Raft.InstallSnapShot", &args, &reply)
	if !ok {
		return
	}
	rf.mu.Lock()
	defer rf.mu.Unlock()
	if reply.Term > rf.currentTerm {
		rf.setNewTerm(reply.Term)
		return
	}
	if rf.state == Leader {
		rf.nextIndex[peer] = rf.LastIndex + 1
		rf.matchIndex[peer] = rf.LastIndex
	}
}

func (rf *Raft) InstallSnapShot(args *SnapShotArgs, reply *SnapShotReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	reply.Term = rf.currentTerm
	if args.Term < rf.currentTerm {
		return
	}
	if args.Term > rf.currentTerm {
		rf.setNewTerm(args.Term)
	}
	rf.state = Follower
	rf.votedFor = args.LeaderID
	rf.persist()
	if args.LastIncludedIndex <= rf.log.Index0 {
		return
	}
	rf.mu.Unlock()
	rf.applyCh <- ApplyMsg{
		SnapshotValid: true,
		Snapshot:      args.SnapShot,
		SnapshotTerm:  args.LastIncludedTerm,
		SnapshotIndex: args.LastIncludedIndex,
	}
	rf.mu.Lock()
}

func (rf *Raft) leaderSendEntries(peer int, args *AppendEntriesArgs) {
	reply := AppendEntriesReply{}
	ok := rf.sendAppendEntries(peer, args, &reply)
	if !ok {
		return
	}
	rf.mu.Lock()
	defer rf.mu.Unlock()
	if reply.Term > rf.currentTerm {
		DPrintf("[%d]: 存在更高的term，转变为Follower", rf.me)
		rf.setNewTerm(reply.Term)
		return
	}
	if rf.state == Leader { //确保当前还处于leader状态
		if reply.Success {
			match := args.PrevLogIndex + len(args.Entries)
			next := match + 1
			rf.nextIndex[peer] = max(rf.nextIndex[peer], next)
			rf.matchIndex[peer] = max(rf.matchIndex[peer], match)
			DPrintf("[%d]: 服务器 %d 对应的 nextIndex 为 $%d#", rf.me, peer, rf.nextIndex[peer])
			DPrintf("[%d]: 服务器 %d 对应的 matchIndex 为 $%d#", rf.me, peer, rf.matchIndex[peer])
		} else if reply.Conflict {
			DPrintf("[%v]: 冲突Index --> %v ，服务器标号 --> %v", rf.me, reply.XIndex, peer)
			if reply.XTerm == -1 { // 日志缺失情况，把next 的开头改为reply.XLen开头。
				rf.nextIndex[peer] = reply.XLen
			} else { // 日志不匹配的情况
				//   leader: 4 4 6 6 6
				// follower: 4 4|5 5

				//   leader: 4 4 6 6 6
				// follower: 4 4|4
				lastLogInXTerm := rf.findLastLogInTerm(reply.XTerm)
				if lastLogInXTerm > 0 {
					rf.nextIndex[peer] = lastLogInXTerm
				} else {
					rf.nextIndex[peer] = reply.XIndex
				}
			}
		}
		rf.Commit()
	}
}

func (rf *Raft) Commit() {
	if rf.state != Leader {
		return
	}
	for n := rf.commitIndex + 1; n <= rf.log.LastLog().Index; n++ {
		if rf.log.at(n).Term != rf.currentTerm {
			continue
		}
		counter := 1
		for serverId := 0; serverId < len(rf.peers); serverId++ {
			if serverId != rf.me && rf.matchIndex[serverId] >= n {
				counter++
			}
			if counter > len(rf.peers)/2 {
				rf.commitIndex = n
				DPrintf("[%v]： Leader提交 日志：index %v", rf.me, rf.commitIndex)
				rf.SendMsg()
				break
			}
		}
	}
}

func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	//只要出错就立即返回，因此先将Success置为false
	reply.Success = false
	reply.Term = rf.currentTerm
	if args.Term < rf.currentTerm {
		return
	}

	if args.Term > rf.currentTerm {
		rf.setNewTerm(args.Term)
	}

	if args.PrevLogIndex < rf.log.Index0 {
		return
	}

	if rf.state == Candidate {
		rf.state = Follower
		rf.persist()
	}
	rf.resetElectionTimer()
	rf.LeaderID = args.LeaderId
	// rule 2
	// 如果本机log索引号小于leader索引号，拒绝接受
	//                PrevLogIndex
	//					  ↓
	//    leader: 3 3 4 5 6 6
	//  follower: 3 3 4
	//  只要follower的日志最新索引小于args的日志索引就适用，包括接收了未被提交的日志等意外情况
	if rf.log.LastLog().Index < args.PrevLogIndex {
		reply.Conflict = true
		reply.XTerm = -1
		reply.XIndex = -1
		reply.XLen = rf.log.Len()
		DPrintf("[%v]: 当前日志索引小于leader日志索引，当前日志长度: %d", rf.me, rf.log.Len())
		DPrintf("[%v]: Conflict XTerm %v, XIndex %v, XLen %v", rf.me, reply.XTerm, reply.XIndex, reply.XLen)
		return
	}

	// 如果本机log的args.PrevLogIndex处的日志的Term与传入参数的Term数不太一样
	// 那么拒绝接受并进行检查日志在哪里不匹配
	//          PrevLogIndex
	//				 ↓
	//   leader: 4 6 6 6
	// follower: 4|5 5
	// 或者是下面的情况
	//            PrevLogIndex
	//			       ↓
	// leader:   4 4 6 6 6
	// follower: 4 4|4 4
	if rf.log.at(args.PrevLogIndex).Term != args.PrevLogTerm {
		reply.Conflict = true
		xTerm := rf.log.at(args.PrevLogIndex).Term
		for xIndex := args.PrevLogIndex; xIndex > 0; xIndex-- {
			// 找到此Term接收的第一个日志的Index
			if rf.log.at(xIndex-1).Term != xTerm {
				reply.XIndex = xIndex
				DPrintf("[%v]: log conflict previous Term is %v", rf.me, rf.log.at(xIndex-1).Term)
				break
			}
		}
		reply.XTerm = xTerm
		reply.XLen = rf.log.Len()
		return
	}

	DPrintf("[%d]: args.Entries %v Local: %v", rf.me, args.Entries, rf.log.Entries)
	length := len(args.Entries)
	for idx := 0; idx < length; idx++ {
		entry := args.Entries[idx]
		if entry.Index <= rf.log.LastLog().Index && rf.log.at(entry.Index).Term != entry.Term {
			rf.log.truncate(entry.Index)
			DPrintf("[%d]: After Truncate, Entry : %v", rf.me, rf.log.Entries)
			rf.persist()
		}
		if entry.Index > rf.log.LastLog().Index {
			rf.log.append(args.Entries[idx:]...)
			rf.persist()
			DPrintf("[%d]: Entry now is %v", rf.me, rf.log.Entries)
			break
		}
	}

	// append entries rpc 5
	// leader 提交后，follower也提交
	if args.LeaderCommit > rf.commitIndex {
		rf.commitIndex = min(args.LeaderCommit, rf.log.LastLog().Index)
		rf.SendMsg()
	}
	reply.Success = true
}

// 4 4 6 6 6
func (rf *Raft) findLastLogInTerm(x int) int {
	for i := rf.log.LastLog().Index; i > 0; i-- {
		term := rf.log.at(i).Term
		if term == x {
			return i
		} else if term < x {
			break
		}
	}
	return -1
}

func (rf *Raft) sendAppendEntries(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
	ok := rf.peers[server].Call("Raft.AppendEntries", args, reply)
	return ok
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a > b {
		return b
	}
	return a
}
