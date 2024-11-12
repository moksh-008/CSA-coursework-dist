package stubs

import "uk.ac.bris.cs/gameoflife/util"

var ServerHandler = "GameOfLifeServerOperations.GOL"
var AliveCellReport = "GameOfLifeServerOperations.AliveCells"
var KeyPressHandler = "GameOfLifeServerOperations.KeyPressHandler"
var ShutDownHandler = "GameOfLifeServer.ShutDown"

type GameState int

//const (
//	Executing GameState = iota
//	Paused
//	Quitting
//)

type AliveCount struct {
	Turn       int
	AliveCells int
}

type Response struct {
	FinalStateWorld    [][]uint8
	AliveCells         []util.Cell
	CompletedTurns     int
	AliveCellCount     int
	NewState           GameState
	InterimAliveCounts []AliveCount
}

type Request struct {
	World       [][]uint8
	NewWorld    [][]uint8
	ImageHeight int
	ImageWidth  int
	Turns       int
}
type KeyPress struct {
	Key rune
}
