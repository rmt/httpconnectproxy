package main

import (
    "flag";
    "fmt";
    "net";
    "io";
    "os";
    "encoding/line";
    "regexp";
)

func Copy(a net.Conn, b net.Conn) {
    // setup one-way forwarding of stream traffic
    io.Copy(a, b)
    // and close both connections when a read fails
    a.Close()
    b.Close()
}

func forward(local net.Conn, remoteAddr string) {
    remote, _ := net.Dial("tcp", "", remoteAddr);
    if remote == nil {
        io.WriteString(local, "HTTP/1.0 502 It's dead, Fred\r\n\r\n")
        local.Close();
        return;
    }
    io.WriteString(local, "HTTP/1.0 200 Connection Established\r\n\r\n")
    go Copy(local, remote);
    go Copy(remote, local);
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
        forward(c, m[1]);
    }
}

func fatal(s string, a ... interface{}) {
    fmt.Fprintf(os.Stderr, "netfwd: %s\n", fmt.Sprintf(s, a...))
    os.Exit(2)
}

func main() {
    remote := "127.0.0.1:8080";
    if len(flag.Args()) != 0 {
        remote = flag.Arg(0)
    }
    netlisten, err := net.Listen("tcp", remote);
    if netlisten == nil {
        fatal("Error: %v", err);
    }
    defer netlisten.Close();

    fmt.Fprintf(os.Stderr, "Listening for HTTP CONNECT's on %s\n", remote);

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
