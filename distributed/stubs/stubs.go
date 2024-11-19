// stubs.go
package stubs

import "uk.ac.bris.cs/gameoflife/util"

// RPC method names
var ServerHandler = "GameOfLifeOperations.GOL"
var AliveCellReport = "GameOfLifeOperations.Alive"
var KeyPresshandler = "GameOfLifeOperations.PressedKey"
var KillServerHandler = "GOLOperations.KillServer"

const (
	Paused    = "Paused"
	Executing = "Executing"
	Quitting  = "Quitting"
)

// Empty request and response for simple RPC calls
type EmptyRequest struct{}
type EmptyResponse struct{}

// WorldState represents the current state of the Game of Life world
type WorldState struct {
	World [][]byte
}

// PauseResumeRequest represents a request to pause or resume the simulation
type PauseResumeRequest struct {
	Pause bool
}

// Response represents the response structure for the Game of Life evolution result
type Response struct {
	FinalWorld                [][]byte    // Final world state after evolution
	CompletedTurns            int         // Number of turns completed
	AliveCellsAfterFinalState []util.Cell // Number of alive cells after the final state
	NewState                  string
}

// Request represents the request structure for initializing the Game of Life simulation
type Request struct {
	InitialWorld [][]byte // Initial state of the world grid
	ImageHeight  int      // Height of the world grid
	ImageWidth   int      // Width of the world grid
	Turns        int      // Number of turns to process
}

// AliveResponse represents the response for the current alive cell count and turn number
type AliveResponse struct {
	AliveCellsCount int // Count of currently alive cells
	Turn            int // Current turn number
}

// AliveRequest represents a request to retrieve the current alive cell count
type AliveRequest struct {
	ImageHeight int // Height of the world grid (used if needed)
	ImageWidth  int // Width of the world grid (used if needed)
}

type KeyResponse struct {
	World [][]byte
	Turns int
}

type KillRequest struct {
}
type KeyRequest struct {
	Key rune
}
type KillResponse struct {
}
