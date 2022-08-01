from __future__ import print_function, division, unicode_literals
import curses
import time
from backports.shutil_get_terminal_size import get_terminal_size
from reprint.reprint import lines_of_content

server_list = []
num_list = []
num = 1
file = open("test-" + str(num) + ".log")
line = ""


class node:
    def __init__(self, index):
        self.CurrentTerm = 0
        self.VoteFor = -1
        self.Logs = []
        self.Commit = 0
        self.success = 0
        self.me = index
        self.state = "follower"
        self.online = 1
        self.running = 1

    def add_log(self, log):
        self.Logs.append(log)

def index_num(elem: node):
    return elem.me


def init_server():
    while True:
        line = file.readline()
        if line.__contains__("RUN"):
            continue
        if not line.__contains__("Initial and Connect"):
            return
        if line.__contains__("Initial and Connect"):
            num = get_index(line)
            if not num_list.__contains__(num):
                server_list.append(node(num))
                num_list.append(num)
        server_list.sort(key=index_num)


def get_index(Line):
    start = Line.index('[') + 1
    end = Line.index(']')
    index = int(Line[start:end])
    return index


def get_command(Line: str):
    num = Line.count("{")
    if num > 1:
        start = Line.index("[{") + 1
        end = len(Line) - 2
        Entry_all = Line[start:end]
        Command_List = []
        for i in range(num):
            tmp_s = Entry_all.index("{") + 1
            tmp_e = Entry_all.index("}")
            entry = Entry_all[tmp_s:tmp_e]
            Entry_all = Entry_all[tmp_e + 1:]

            arr = entry.split(" ")
            command = arr[0]
            term = arr[1]
            ind = arr[2]
            Command_List.append({"Command": command, "Term": term, "Index": ind})
        return Command_List
    elif num == 1:
        start = Line.index("{")
        end = Line.index("}")
        Entry = Line[start + 1:end]
        arr = Entry.split(" ")
        command = arr[0]
        term = arr[1]
        ind = arr[2]
        return {"Command": command, "Term": term, "Index": ind}


def get_term(line):
    start = line.index('$') + 1
    end = line.index('#')
    term = int(line[start:end])
    return term



'''

def status_show(line):
    # Log Term
    with output(initial_len=2 * len(server_list) + 1, interval=0) as output_lines:
        for i in range(len(server_list)):
            now = server_list[i]
            assert isinstance(now, node)
            Logterm = ""
            for log in now.Logs:
                Logterm += "| {} ".format(log["Term"])

            fill_space = ''
            if now.state == "leader":
                fill_space = '-' * 3
            elif now.state == "follower":
                fill_space = '-'
            elif now.state == "candidate":
                fill_space = ''
            output_lines[i] = "{}--{}{}: {}".format(i, now.state, fill_space, Logterm)
        # Log Command
        output_lines[len(server_list)] = "\n"
        for i in range(len(server_list)):
            now = server_list[i]
            assert isinstance(now, node)
            Logterm = ""
            for log in now.Logs:
                Logterm += "| {} ".format(log["Command"])
                fill_space = ''
            if now.state == "leader":
                fill_space = '-' * 3
            elif now.state == "follower":
                fill_space = '-'
            elif now.state == "candidate":
                fill_space = ''
            output_lines[i + len(server_list) + 1] = "{}--{}{}: {}".format(i, now.state, fill_space, Logterm)
        # output_lines[2 * len(server_list) + 1] = '\n'
        # output_lines[2 * len(server_list) + 2] = "{}".format(line)
'''


def status_show(line):
    # Log Term
    line_now = 0
    columns, rows = get_terminal_size()
    for i in range(len(server_list)):
        now = server_list[i]
        assert isinstance(now, node)
        Logterm = ""
        tmp = now.Logs[now.success:]
        for log in tmp:
            Logterm += "| {} ".format(log["Term"])
        fill_space = ''
        if now.state == "leader":
            fill_space = '-' * 3
        elif now.state == "follower":
            fill_space = '-'
        elif now.state == "candidate":
            fill_space = ''
        str = "{}--{}--{}Term--{}: {}".format(i, now.state, fill_space, now.CurrentTerm, Logterm)
        lines = int((len(str) - 1)/columns) + 1
        stdscr.addstr(line_now, 0, "{}\n".format(str))
        line_now += lines
        # stdscr.addstr("{}".format(str))
        stdscr.refresh()
    # Log Command

    str = "\n"
    lines = int((len(str) - 1)/columns) + 1
    stdscr.addstr(line_now, 0, "{}\n".format(str))
    line_now += lines

    str = "{}\n".format('--------------')
    lines = int((len(str) - 1)/columns) + 1
    stdscr.addstr(line_now, 0, "{}\n".format(str))
    line_now += lines

    str = "\n"
    lines = int((len(str) - 1)/columns) + 1
    stdscr.addstr(line_now, 0, "{}\n".format(str))
    line_now += lines
    # stdscr.addstr("{}".format('--------------'))
    stdscr.refresh()
    for i in range(len(server_list)):
        now = server_list[i]
        assert isinstance(now, node)
        Logterm = ""
        tmp = now.Logs[now.success:]
        for log in tmp:
            Logterm += "| {} ".format(log["Command"])
            fill_space = ''
        if now.state == "leader":
            fill_space = '-' * 3
        elif now.state == "follower":
            fill_space = '-'
        elif now.state == "candidate":
            fill_space = ''
        str = "{}--{}{}: {}".format(i, now.state, fill_space, Logterm)
        lines = int((len(str) - 1)/columns) + 1
        stdscr.addstr(line_now, 0, "{}\n".format(str))
        line_now += lines
        # stdscr.addstr("{}".format(str))
        stdscr.refresh()
    change_line = '\n'
    lines = int((len(change_line) - 1)/columns) + 1
    stdscr.addstr(line_now, 0, "{}\n".format(change_line))
    line_now += lines


init_server()
stdscr = curses.initscr()
stdscr.scrollok(1)
curses.noecho()
curses.cbreak()

while True:
    if line.__contains__("请求投票"):
        index = get_index(line)
        server_list[index].state = "candidate"
        term = get_term(line)
        server_list[index].CurrentTerm = int(term)
    elif line.__contains__("Term 设置为$%d#"):
        index = get_index(line)
        server_list[index].CurrentTerm = get_term(line)
        server_list[index].state = "follower"
    elif line.__contains__("投票过半，提前结束"):
        index = get_index(line)
        server_list[index].state = "leader"
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
    elif line.__contains__("Entry now is"):
        index = get_index(line)
        Entry = get_command(line)
        server_list[index].state = "follower"
        server_list[index].Logs = Entry
    elif line.__contains__(": COMMIT"):
        index = get_index(line)
        log_index = int(line[line.index('$') + 1:line.index('#')])
        server_list[index].success = log_index
    status_show(line)
    line = file.readline()
    time.sleep(0.01)

'''
import curses
import time

def report_progress(filename, progress):
    """progress: 0-10"""
    stdscr.addstr(0, 0, "Moving file: {0}".format(filename))
    stdscr.addstr(1, 0, "Total progress: [{1:10}] {0}%".format(progress * 10, "#" * progress))
    stdscr.refresh()

if __name__ == "__main__":
    stdscr = curses.initscr()
    curses.noecho()
    curses.cbreak()

    try:
        for i in range(10):
            report_progress("file_{0}.txt".format(i), i+1)
            time.sleep(0.5)
    finally:
        curses.echo()
        curses.nocbreak()
        curses.endwin()
'''
