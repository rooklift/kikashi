package main

// This program exists to MITM the Lizzie / LZ comms for research.

import (
	"bufio"
	"io"
	"os"
	"os/exec"
)

const REAL_PROGRAM = "./real_leelaz.exe"

var log_chan = make(chan []byte, 128)

func main() {

	exec_command := exec.Command(REAL_PROGRAM, os.Args[1:]...)
	i_pipe, _ := exec_command.StdinPipe()
	o_pipe, _ := exec_command.StdoutPipe()
	e_pipe, _ := exec_command.StderrPipe()

	exec_command.Start()

	go mitm(os.Stdin, i_pipe, []byte("Lizzie: "))
	go mitm(o_pipe, os.Stdout, []byte("STDOUT: "))
	go mitm(e_pipe, os.Stderr, []byte("stderr: "))
	logger()
}

func mitm(input io.Reader, output io.Writer, prefix []byte) {

	scanner := bufio.NewScanner(input)

	for scanner.Scan() {

		output.Write(scanner.Bytes())
		output.Write([]byte{'\n'})

		log_message := make([]byte, len(prefix) + len(scanner.Bytes()))
		copy(log_message, prefix)
		copy(log_message[len(prefix):], scanner.Bytes())
		log_chan <- log_message
	}
}

func logger() {

	outfile, _ := os.Create("mitm_log.txt")

	for {
		b := <- log_chan
		outfile.Write(b)
		outfile.Write([]byte{'\n'})
	}
}
