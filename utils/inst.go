package utils

import (
	"gitlab.com/akita/mem/v3/vm"
	"gitlab.com/akita/mgpusim/v3/insts"
//	"gitlab.com/akita/mgpusim/v3/utils"
)

// InstEmuState is the interface used by the emulator to track the instruction
// execution status.
type InstEmuState interface {
	PID() vm.PID
	Inst() *insts.Inst
	Scratchpad() Scratchpad
    GetPC() uint64
}
