package main

import (
	"flag"
	"io"
	"log"
	"math/rand"
	"net"
	"sync"
	"time"
)

var (
	dropRate  float64
	listenTCP string
	targetTCP string
	listenUDP string
	targetUDP string
)

func init() {
	flag.Float64Var(&dropRate, "drop", 0.15, "Probability of dropping connection (0.0 to 1.0)")
	flag.StringVar(&listenTCP, "listen-tcp", ":443", "TCP port to listen on (Reality facade)")
	flag.StringVar(&targetTCP, "target-tcp", "127.0.0.1:8443", "Target Reality server")
	flag.StringVar(&listenUDP, "listen-udp", ":443", "UDP port to listen on (Hysteria facade)")
	flag.StringVar(&targetUDP, "target-udp", "127.0.0.1:9443", "Target Hysteria server")
	rand.Seed(time.Now().UnixNano())
}

func main() {
	flag.Parse()
	log.Printf("Starting Mock DPI Injector. Drop rate: %.0f%%", dropRate*100)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		startTCPProxy()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		startUDPProxy()
	}()

	wg.Wait()
}

func startTCPProxy() {
	l, err := net.Listen("tcp", listenTCP)
	if err != nil {
		log.Fatalf("Failed to listen on TCP %s: %v", listenTCP, err)
	}
	defer l.Close()
	log.Printf("Listening for TCP on %s -> %s", listenTCP, targetTCP)

	for {
		conn, err := l.Accept()
		if err != nil {
			log.Printf("TCP accept error: %v", err)
			continue
		}

		go handleTCP(conn)
	}
}

func handleTCP(src net.Conn) {
	defer src.Close()

	// DPI Check: Do we inject a RST?
	if rand.Float64() < dropRate {
		log.Printf("[DPI-TCP] Injecting RST for %s", src.RemoteAddr())
		// Set a deadline in the past to immediately fail/close, simulating RST
		if tcpConn, ok := src.(*net.TCPConn); ok {
			tcpConn.SetLinger(0) // Force RST on close
		}
		return
	}

	dst, err := net.DialTimeout("tcp", targetTCP, 2*time.Second)
	if err != nil {
		log.Printf("TCP target unreachable: %v", err)
		return
	}
	defer dst.Close()

	log.Printf("[TCP] Forwarding %s -> %s", src.RemoteAddr(), targetTCP)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		io.Copy(dst, src)
	}()

	go func() {
		defer wg.Done()
		io.Copy(src, dst)
	}()

	wg.Wait()
}

func startUDPProxy() {
	srcAddr, err := net.ResolveUDPAddr("udp", listenUDP)
	if err != nil {
		log.Fatalf("Failed to resolve UDP listen: %v", err)
	}

	dstAddr, err := net.ResolveUDPAddr("udp", targetUDP)
	if err != nil {
		log.Fatalf("Failed to resolve UDP target: %v", err)
	}

	l, err := net.ListenUDP("udp", srcAddr)
	if err != nil {
		log.Fatalf("Failed to listen on UDP %s: %v", listenUDP, err)
	}
	defer l.Close()
	log.Printf("Listening for UDP on %s -> %s", listenUDP, targetUDP)

	buf := make([]byte, 65535)
	for {
		n, raddr, err := l.ReadFromUDP(buf)
		if err != nil {
			continue
		}

		if rand.Float64() < dropRate {
			log.Printf("[DPI-UDP] Dropping packet from %s", raddr)
			continue // Blackhole the packet
		}

		// Simple stateless forwarding (not a full NAT, but works for basic test)
		conn, err := net.DialUDP("udp", nil, dstAddr)
		if err == nil {
			conn.Write(buf[:n])
			// Read one response back (simplification for testing)
			conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
			rn, _ := conn.Read(buf)
			if rn > 0 {
				l.WriteToUDP(buf[:rn], raddr)
			}
			conn.Close()
		}
	}
}
