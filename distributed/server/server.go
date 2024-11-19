// server.go
package main

import (
	"flag"
	"fmt"
	"math/rand"
	"net"
	"net/rpc"
	"sync"
	"time"
	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

var (
	GolWorld [][]byte
	GolTurn  int
	Pause    string = "Continue"
	Quit     string = "No"
	Close    string = "No"
	mu       sync.Mutex
	KillChan = make(chan bool)
)

func makeWorld(height int, width int) [][]byte {
	world := make([][]byte, height)
	for i := range world {
		world[i] = make([]byte, width)
	}
	return world
}

// GameOfLifeOperations struct that serves the RPC methods
type GameOfLifeOperations struct {
	isPaused bool
	Mu       sync.Mutex
	World    [][]byte
	Turns    int
	Quit     bool
	Paused   bool
	Workers  []*rpc.Client
}

// GOL processes the Game of Life evolution for the specified number of turns.
func (s *GameOfLifeOperations) GOL(req stubs.Request, res *stubs.Response) (err error) {

	GolWorld = req.InitialWorld
	height := req.ImageHeight
	width := req.ImageWidth
	turns := req.Turns

	// Process each turn, evolving the world state
	for t := 0; t < turns; t++ {

		if Quit == "Yes" {
			fmt.Println("Received quit signal. Ending simulation.")
			break
		}

		mu.Lock()
		GolWorld = executeTurn(GolWorld, height, width)
		GolTurn = t + 1 // Update the global turn count
		mu.Unlock()

		for Pause == "Pause" {
			time.Sleep(1 * time.Second)
		}
	}

	// Populate the response with the final world state and alive cells after final state
	res.FinalWorld = GolWorld
	res.CompletedTurns = GolTurn
	res.AliveCellsAfterFinalState = findAliveCells(GolWorld)

	return
}

// Alive provides the count of currently alive cells and the current turn
func (s *GameOfLifeOperations) Alive(req stubs.AliveRequest, res *stubs.AliveResponse) (err error) {

	for Pause == "Pause" {
		time.Sleep(1 * time.Second)
	}

	// Calculate the alive cells based on the current world state
	mu.Lock()
	res.Turn = GolTurn
	res.AliveCellsCount = countAliveCells(GolWorld)
	mu.Unlock()
	return
}

func (s *GameOfLifeOperations) KillServer(req stubs.KillRequest, res *stubs.KillResponse) (err error) {
	KillChan <- true
	return
}

func (s *GameOfLifeOperations) PressedKey(req stubs.KeyRequest, res *stubs.KeyResponse) (err error) {

	res.Turns = GolTurn
	res.World = GolWorld
	switch req.Key {
	case 'p':
		if s.Paused == false {
			s.Paused = true
		} else {
			s.Paused = false
		}
	case 'q':
		s.Quit = true
	case 'k':
		s.Quit = true
	}
	return
}

// executeTurn performs a single evolution of the Game of Life
func executeTurn(world [][]byte, height, width int) [][]byte {
	newWorld := make([][]byte, height)
	for i := range newWorld {
		newWorld[i] = make([]byte, width)
	}

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			aliveNeighbors := countAliveNeighbors(world, x, y, height, width)
			currentCell := world[y][x]

			if currentCell == 255 {

				if aliveNeighbors < 2 || aliveNeighbors > 3 {
					newWorld[y][x] = 0
				} else {
					newWorld[y][x] = 255
				}
			} else {
				// Cell is currently dead
				if aliveNeighbors == 3 {
					newWorld[y][x] = 255
				} else {
					newWorld[y][x] = 0
				}
			}
		}
	}

	return newWorld
}

// countAliveNeighbors counts alive neighbors for a cell at (x, y)
func countAliveNeighbors(world [][]byte, x, y, height, width int) int {
	liveNeighbors := 0
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if dx == 0 && dy == 0 {
				continue
			}
			neighborY := (y + dy + height) % height
			neighborX := (x + dx + width) % width
			if world[neighborY][neighborX] == 255 {
				liveNeighbors++
			}
		}
	}
	return liveNeighbors
}

// countAliveCells counts the number of alive cells in the world
func countAliveCells(world [][]byte) int {
	aliveCount := 0
	for y := 0; y < len(world); y++ {
		for x := 0; x < len(world[y]); x++ {
			if world[y][x] == 255 {
				aliveCount++
			}
		}
	}
	return aliveCount
}

// findAliveCells collects the coordinates of all live cells in the world
func findAliveCells(world [][]byte) []util.Cell {
	aliveCells := []util.Cell{}
	for y := 0; y < len(world); y++ {
		for x := 0; x < len(world[y]); x++ {
			if world[y][x] == 255 {
				aliveCells = append(aliveCells, util.Cell{Y: y, X: x})
			}
		}
	}
	return aliveCells
}

func main() {
	// Initialize the Game of Life RPC server
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	rand.Seed(time.Now().UnixNano())
	rpc.Register(&GameOfLifeOperations{})
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	defer listener.Close()
	fmt.Println("Server started on port", *pAddr)
	rpc.Accept(listener)
}
