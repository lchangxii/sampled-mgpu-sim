package cu

import (
//	"log"

//	"gitlab.com/akita/mem/v3/mem"
	"gitlab.com/akita/mem/v3/vm"
)

type storageAccessor interface {

Read(pid vm.PID, vAddr, byteSize uint64) []byte
Write(pid vm.PID, vAddr uint64, data []byte)
}

//func newStorageAccessor(
//	storage *mem.Storage,
//	pageTable vm.PageTable,
//	log2PageSize uint64,
//	addrConverter mem.AddressConverter,
//) *storageAccessor {
//	a := new(storageAccessor)
//	a.storage = storage
//	a.addrConverter = addrConverter
//	a.pageTable = pageTable
//	a.log2PageSize = log2PageSize
//	return a
//}
