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
		}
	}
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
		DPrintf("[%d]: find new term, state --> follower", rf.me)
		rf.setNewTerm(reply.Term)
		return
	}
	if rf.state == Leader { //确保当前还处于leader状态
		if reply.Success {
			match := args.PrevLogIndex + len(args.Entries)
			next := match + 1
			rf.nextIndex[peer] = max(rf.nextIndex[peer], next)
			rf.matchIndex[peer] = max(rf.matchIndex[peer], match)
		} else if reply.Conflict {
			DPrintf("[%v]: 冲突Index --> %v ，服务器标号 --> %v", rf.me, reply.XIndex, peer)
			if reply.XTerm == -1 { // 日志缺失情况，把next 的开头改为reply.XLen开头。
				rf.nextIndex[peer] = reply.XLen
			} else {
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

	if args.Term < rf.currentTerm {
		reply.Success = false
		reply.Term = rf.currentTerm
		return
	}

	if args.Term > rf.currentTerm {
		rf.setNewTerm(args.Term)
	}
	if rf.state == Candidate {
		rf.state = Follower
	}
	// rule 2
	// 如果本机log索引号小于leader索引号，拒绝接受
	//    leader: 3 3 4 5 6 6 6
	//  follower: 3 3 4
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

	//   leader: 4 6 6 6
	// follower: 4 5 5
	if rf.log.at(args.PrevLogIndex).Term != args.PrevLogTerm {
		reply.Conflict = true
		xTerm := rf.log.at(args.PrevLogIndex).Term
		for xIndex := args.PrevLogIndex; xIndex > 0; xIndex-- {
			//   leader: 4 6 6 6
			// follower: 4 5 5
			if rf.log.at(xIndex-1).Term != xTerm {
				reply.XIndex = xIndex
				DPrintf("[%v]: log conflict previous Term is %v", rf.me, rf.log.at(xIndex-1).Term)
				break
			}
		}
		//   leader: 4 4 6 6 6
		// follower: 4 4 4 4
		reply.XTerm = xTerm
		reply.XLen = rf.log.Len()
		return
	}

	DPrintf("[%d]: \nargs.Entries: %v\nLocal: %v\n", rf.me, args.Entries, rf.log.Entries)

	/*
		if len(args.Entries) > 0 {
			entry := args.Entries[0]
			if entry.Index <= rf.log.LastLog().Index && rf.log.at(entry.Index).Term != entry.Term {
				rf.log.truncate(entry.Index)
				rf.persist()
			}
			if entry.Index > rf.log.LastLog().Index {
				rf.log.append(args.Entries[0:]...)
				if entry.Index == 200 {
					DPrintf("debug code")
				}
				DPrintf("[%d]: entry now is\n %v", rf.me, rf.log.Entries)

				rf.persist()
			}
		}

	*/

	for idx := 0; idx < len(args.Entries); idx++ {
		entry := args.Entries[idx]
		if entry.Index <= rf.log.LastLog().Index && rf.log.at(entry.Index).Term != entry.Term {
			rf.log.truncate(entry.Index)
			rf.persist()
		}
		if entry.Index > rf.log.LastLog().Index {
			rf.log.append(args.Entries[idx:]...)
			if entry.Index == 200 {
				DPrintf("debug code")
			}
			DPrintf("[%d]: entry now is\n %v", rf.me, rf.log.Entries)

			rf.persist()
		}
	}

	/*
		for idx, entry := range args.Entries {
			// append entries rpc 3
			DPrintf("[%d]: LOOP IN APPEND ENTRY;\n entry.Index --> %d, lastlog.Index --> %d"+
				"; entry.Term --> %d, lastlog.Term --> %d", rf.me, entry.Index, rf.log.LastLog().Index,
				entry.Term, rf.log.LastLog().Term)
			if entry.Index <= rf.log.LastLog().Index && rf.log.at(entry.Index).Term != entry.Term {
				rf.log.truncate(entry.Index)
				rf.persist()
			}
			// append entries rpc 4
			if entry.Index > rf.log.LastLog().Index {
				rf.log.append(args.Entries[idx:]...)
				if entry.Index == 200 {
					DPrintf("debug code")
				}
				DPrintf("[%d]: after append entry %v\n local log is\n %v", rf.me, args.Entries[idx:], rf.log.Entries)
				rf.persist()
				break
			}
		}

	*/

	// append entries rpc 5
	if args.LeaderCommit > rf.commitIndex {
		rf.commitIndex = min(args.LeaderCommit, rf.log.LastLog().Index)
		rf.SendMsg()
	}
	reply.Success = true
	rf.resetElectionTimer()
}

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
