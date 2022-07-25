server_list = []
num_list = []
num = 3
file = open("test-" + str(num) + ".log")
line = ""

class node:
    def __init__(self, index):
        self.CurrentTerm = 0
        self.VoteFor = -1
        self.Logs = []
        self.Commit = 0
        self.suceess = 0
        self.me = index
        self.state = "follower"
        self.online = 1
        self.running = 1

    def log_get(self):
        tmp = self.Logs[self.suceess:]
        for log in tmp:
            if int(log["Index"]) <= self.suceess:
                str = "C:{} T:{} I:{}*|".format(log["Command"], log["Term"], log["Index"])
            else:
                str = "C:{} T:{} I:{} |".format(log["Command"], log["Term"], log["Index"])
            if self.state == "follower":
                print('\033[94m' + str, end="")
            elif self.state == "candidate":
                print('\033[93m' + str, end="")
            elif self.state == "leader":
                print('\033[92m' + str, end="")
        print("")

    def add_log(self, log):
        self.Logs.append(log)

    def print_log(self):
        tmp = self.Logs[self.suceess:]
        if self.online == 1:
            if self.state == "follower":
                print('\033[94m' + str(self.me) + "--" + self.state + "--Term: "+str(self.CurrentTerm)+" --> ", end=" ")
            elif self.state == "candidate":
                print('\033[93m' + str(self.me) + "--" + self.state+ "-Term: "+str(self.CurrentTerm)+" --> ", end=" ")
            elif self.state == "leader":
                print('\033[92m' +str(self.me) + "--" + self.state+ "----Term: "+str(self.CurrentTerm)+" --> ", end=" ")
            self.log_get()

        elif self.online == 0:
            print('\033[91m' + str(self.me) + "--" + self.state, end=" ")
            self.log_get()


def index_num(elem: node):
    return elem.me


def init_server():
    while True:
        line = file.readline()
        if line.__contains__("目前Term"):
            return
        if line.__contains__("function LoopCommit"):
            num = int(line[1])
            if not num_list.__contains__(num):
                server_list.append(node(num))
                num_list.append(num)
        server_list.sort(key=index_num)


def get_index(Line):
    return int(Line[1])


def get_command(Line: str):
    num = Line.count("{")
    if num > 1:
        start = Line.index("[{")+1
        end = len(Line)-2
        Entry_all=Line[start:end]
        Command_List = []
        for i in range(num):
            tmp_s = Entry_all.index("{")+1
            tmp_e = Entry_all.index("}")
            entry = Entry_all[tmp_s:tmp_e]
            Entry_all=Entry_all[tmp_e+1:]

            arr = entry.split(" ")
            command = arr[0]
            term = arr[1]
            ind = arr[2]
            commit = 0
            Command_List.append({"Command":command, "Term":term, "Index":ind,"IfCommit":0})
        return Command_List
    elif num == 1:
        start = Line.index("{")
        end = Line.index("}")
        Entry = Line[start+1:end]
        arr = Entry.split(" ")
        command = arr[0]
        term = arr[1]
        ind = arr[2]
        return {"Command":command, "Term":term, "Index":ind, "IfCommit":0}


def get_term(line):
    return int(line[len(line) - 2])


def status_show():
    for node in server_list:
        node.print_log()
    print(" ")


init_server()
while True:
    if line.__contains__("请求投票"):
        index = get_index(line)
        server_list[index].state = "candidate"
        pos = line.index("，")
        term = line[pos-1]
        server_list[index].CurrentTerm = int(term)
        status_show()
    elif line.__contains__("设置term为"):
        index = get_index(line)
        server_list[index].CurrentTerm = get_term(line)
        server_list[index].state = "follower"
        status_show()
    elif line.__contains__("投票过半，提前结束"):
        index = get_index(line)
        server_list[index].state = "leader"
        status_show()
    elif line.__contains__("TEST: server disconnect"):
        index = get_index(line)
        server_list[index].online = 0
    elif line.__contains__("TEST : server reconnect"):
        index = get_index(line)
        server_list[index].online = 1
    elif line.__contains__("TEST: function one finished"):
        continue
    elif line.__contains__("Leader entry now is"):
        index = get_index(line)
        Entry = get_command(line)
        server_list[index].Logs = Entry
        status_show()
    elif line.__contains__("entry now is"):
        index = get_index(line)
        Entry = get_command(line)
        server_list[index].state = "follower"
        server_list[index].Logs = Entry
        status_show()
    elif line.__contains__(": COMMIT"):
        index = get_index(line)
        pos = line.index('COMMIT')
        log_index = line[pos + 7]
        server_list[index].Logs[int(log_index)]["IfCommit"] = 1
        server_list[index].success = log_index

    line = file.readline()