package main

import (
    "flag";
    "fmt";
    "net";
    "io";
    "os";
    "encoding/line";
    "regexp";
    "exec";
    "syscall";
)

var portspec string
var command string

type MyCmd exec.Cmd

func (c MyCmd) Read(p []byte) (n int, err os.Error) {
    return c.Stdout.Read(p)
}
func (c MyCmd) Write(p []byte) (n int, err os.Error) {
    return c.Stdin.Write(p)
}
func (c MyCmd) Close() os.Error {
    // send SIGHUP then close in the normal way
    if c.Pid > 0 {
        syscall.Kill(c.Pid, syscall.SIGHUP)
    }
    cmd := exec.Cmd(c)
    return cmd.Close()
}

func Copy(a io.ReadWriteCloser, b io.ReadWriteCloser) {
    // setup one-way forwarding of stream traffic
    io.Copy(a, b)
    // and close both connections when a read fails
    a.Close()
    b.Close()
}

func forward(local net.Conn, remoteAddr string) {
    var laddr *net.TCPAddr
    raddr, err := net.ResolveTCPAddr(remoteAddr)
    if err != nil {
        io.WriteString(local, "HTTP/1.0 502 That's no street, Pete\r\n\r\n")
        local.Close()
        return
    }
    remote, _ := net.DialTCP("net", laddr, raddr);
    if remote == nil {
        io.WriteString(local, "HTTP/1.0 502 It's dead, Fred\r\n\r\n")
        local.Close()
        return
    }
    remote.SetKeepAlive(true)
    io.WriteString(local, "HTTP/1.0 200 Connection Established\r\n\r\n")
    go Copy(local, remote);
    go Copy(remote, local);
}

func forward2cmd(local net.Conn, remoteAddr string) {
    remoteenvstr := "REMOTE=" + local.RemoteAddr().String();
    cwd, err := os.Getwd(); if err != nil { cwd = "/" }
    remote, _ := exec.Run(command, []string{command, remoteAddr},
        []string{remoteenvstr}, cwd,
        exec.Pipe, exec.Pipe, exec.DevNull)
    if remote == nil {
        io.WriteString(local, "HTTP/1.0 502 It's dead, Fred\r\n\r\n")
        local.Close();
        return;
    }
    io.WriteString(local, "HTTP/1.0 200 Connection Established\r\n\r\n")
    go Copy(local, MyCmd(*remote));
    go Copy(MyCmd(*remote), local);
}

func newconn(c net.Conn) {
    // find out the desired destination on a new connect
    connre := regexp.MustCompile("CONNECT (.*) HTTP/")
    r := line.NewReader(c, 512);
    l, isprefix, err := r.ReadLine()
    if err != nil || isprefix == true {
        c.Close();
        return;
    }
    m := connre.FindStringSubmatch(string(l));
    if m == nil {
        io.WriteString(c, "HTTP/1.0 502 No luck, Chuck\r\n\r\n")
        c.Close();
        return;
    }
    // wait until we get a blank line (end of HTTP headers)
    for {
        l, _, _ := r.ReadLine();
        if l == nil { return; }
        if len(l) == 0 { break; }
    }
    if l != nil {
        if(command == "") {
            forward(c, m[1]);
        } else {
            forward2cmd(c, m[1]);
        }
    }
}

func fatal(s string, a ... interface{}) {
    fmt.Fprintf(os.Stderr, "netfwd: %s\n", fmt.Sprintf(s, a...))
    os.Exit(2)
}

func main() {
    flag.StringVar(&command, "E", "", "Executable to run with CONNECT string as argument")
    flag.StringVar(&portspec, "P", "127.0.0.1:8080", ":port or ip:port to listen on.")
    flag.Parse()
    netlisten, err := net.Listen("tcp", portspec);
    if netlisten == nil {
        fatal("Error: %v", err);
    }
    defer netlisten.Close();

    fmt.Fprintf(os.Stderr, "Listening for HTTP CONNECT's on %s\n", portspec);

    for {
        // wait for clients
        conn, err := netlisten.Accept();
        if conn != nil {
            go newconn(conn)
        } else {
            fatal("Error: %v", err);
        }
    }
}
