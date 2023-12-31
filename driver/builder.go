package driver

import (
	"gitlab.com/akita/akita/v3/sim"
	"gitlab.com/akita/mem/v3/mem"
	"gitlab.com/akita/mgpusim/v3/utils"
	"gitlab.com/akita/mem/v3/vm"
	"gitlab.com/akita/mgpusim/v3/driver/internal"
)

// A Builder can build a driver.
type Builder struct {
	engine             sim.Engine
	freq               sim.Freq
	log2PageSize       uint64
	pageTable          vm.PageTable
	globalStorage      *mem.Storage
	useMagicMemoryCopy bool
}

// MakeBuilder creates a driver builder with some default configuration
// parameters.
func MakeBuilder() Builder {
	return Builder{
		freq: 1 * sim.GHz,
	}
}

// WithEngine sets the engine to use.
func (b Builder) WithEngine(e sim.Engine) Builder {
	b.engine = e
	return b
}

// WithFreq sets the frequency to use.
func (b Builder) WithFreq(freq sim.Freq) Builder {
	b.freq = freq
	return b
}

// WithPageTable sets the global page table.
func (b Builder) WithPageTable(pt vm.PageTable) Builder {
	b.pageTable = pt
	return b
}

// WithLog2PageSize sets the page size used by all the devices in the system
// as a power of 2.
func (b Builder) WithLog2PageSize(log2PageSize uint64) Builder {
	b.log2PageSize = log2PageSize
	return b
}

// WithGlobalStorage sets the global storage that the driver uses.
func (b Builder) WithGlobalStorage(storage *mem.Storage) Builder {
	b.globalStorage = storage
	return b
}

// WithMagicMemoryCopyMiddleware uses global storage as memory components
func (b Builder) WithMagicMemoryCopyMiddleware() Builder {
	b.useMagicMemoryCopy = true
	return b
}

// Build creates a driver.
func (b Builder) Build(name string) *Driver {
	driver := new(Driver)
	driver.TickingComponent = sim.NewTickingComponent(
		"driver", b.engine, b.freq, driver)

	driver.Log2PageSize = b.log2PageSize

	memAllocatorImpl := internal.NewMemoryAllocator(b.pageTable, b.log2PageSize)
	driver.memAllocator = memAllocatorImpl

	distributorImpl := newDistributorImpl(memAllocatorImpl)
	distributorImpl.pageSizeAsPowerOf2 = b.log2PageSize
	driver.distributor = distributorImpl

	driver.pageTable = b.pageTable
	driver.globalStorage = b.globalStorage

	if b.useMagicMemoryCopy {
		globalStorageMemoryCopyMiddleware := &globalStorageMemoryCopyMiddleware{
			driver: driver,
		}
		driver.middlewares = append(driver.middlewares, globalStorageMemoryCopyMiddleware)
	} else {
		defaultMemoryCopyMiddleware := &defaultMemoryCopyMiddleware{
			driver: driver,
		}
		driver.middlewares = append(driver.middlewares, defaultMemoryCopyMiddleware)
	}

	driver.gpuPort = sim.NewLimitNumMsgPort(driver, 40960000, "driver.ToGPUs")
	driver.AddPort("GPU", driver.gpuPort)
	driver.mmuPort = sim.NewLimitNumMsgPort(driver, 1, "driver.ToMMU")
	driver.AddPort("MMU", driver.mmuPort)

	driver.enqueueSignal = make(chan bool)
	driver.driverStopped = make(chan bool)

	b.createCPU(driver)

	return driver
}

func (b *Builder) createCPU(d *Driver) {
	cpu := &internal.Device{
		ID:       0,
		Type:     internal.DeviceTypeCPU,
		MemState: internal.NewDeviceMemoryState(d.Log2PageSize),
	}
    if *utils.ArchFlag == "mi100"{
	    cpu.SetTotalMemSize(32 * mem.GB)
    } else {

	    cpu.SetTotalMemSize(4 * mem.GB)
    }

	d.memAllocator.RegisterDevice(cpu)
	d.devices = append(d.devices, cpu)
}
