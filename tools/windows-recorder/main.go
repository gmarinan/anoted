// Windows native audio capture helper for meetctl (WSL2 and native Windows).
//
// Communicates via JSON lines on stdin/stdout.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

type request struct {
	Cmd string `json:"cmd"`
}

type response struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
	State string `json:"state,omitempty"`
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "status" {
		emit(response{OK: true, State: "idle"})
		return
	}

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		var req request
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			emit(response{OK: false, Error: err.Error()})
			continue
		}
		switch req.Cmd {
		case "status":
			emit(response{OK: true, State: "idle"})
		case "list_devices":
			emit(response{OK: true, State: "not_implemented"})
		case "start":
			emit(response{OK: false, Error: "WASAPI capture not yet implemented"})
		case "stop":
			emit(response{OK: true, State: "idle"})
		default:
			emit(response{OK: false, Error: "unknown command"})
		}
	}
}

func emit(resp response) {
	_ = json.NewEncoder(os.Stdout).Encode(resp)
	fmt.Println()
}
