package pcap

import (
	"io"
	"os"
	"strconv"
	"sync"
	"time"

	"gitlab.x.lan/yunshan/droplet-libs/queue"
	"gitlab.x.lan/yunshan/droplet-libs/stats"
)

type WorkerManager struct {
	inputQueue queue.MultiQueueReader
	nQueues    int

	tcpipChecksum         bool
	blockSizeKB           int
	maxConcurrentFiles    int
	maxFileSizeMB         int
	maxFilePeriodSecond   int
	maxDirectorySizeGB    int
	diskFreeSpaceMarginGB int
	maxFileKeepDay        int
	baseDirectory         string
}

func NewWorkerManager(
	inputQueue queue.MultiQueueReader,
	nQueues int,
	tcpipChecksum bool,
	blockSizeKB int,
	maxConcurrentFiles int,
	maxFileSizeMB int,
	maxFilePeriodSecond int,
	maxDirectorySizeGB int,
	diskFreeSpaceMarginGB int,
	maxFileKeepDay int,
	baseDirectory string,
) *WorkerManager {
	return &WorkerManager{
		inputQueue: inputQueue,
		nQueues:    nQueues,

		tcpipChecksum:         tcpipChecksum,
		blockSizeKB:           blockSizeKB,
		maxConcurrentFiles:    maxConcurrentFiles,
		maxFileSizeMB:         maxFileSizeMB,
		maxFilePeriodSecond:   maxFilePeriodSecond,
		maxDirectorySizeGB:    maxDirectorySizeGB,
		diskFreeSpaceMarginGB: diskFreeSpaceMarginGB,
		maxFileKeepDay:        maxFileKeepDay,
		baseDirectory:         baseDirectory,
	}
}

func (m *WorkerManager) Start() []io.Closer {
	os.MkdirAll(m.baseDirectory, os.ModePerm)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go markAndCleanTempFiles(m.baseDirectory, wg)
	wg.Wait()

	NewCleaner(int64(m.maxDirectorySizeGB)<<30, int64(m.diskFreeSpaceMarginGB)<<30, time.Duration(m.maxFileKeepDay)*time.Hour*24, m.baseDirectory).Start()
	closers := make([]io.Closer, m.nQueues)
	for i := 0; i < m.nQueues; i++ {
		worker := m.newWorker(i)
		stats.RegisterCountable("pcap", worker, stats.OptionStatTags{"index": strconv.Itoa(i)})
		closers[i] = io.Closer(worker)
		go worker.Process()
	}
	return closers
}
