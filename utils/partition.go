package utils

// IdxRange Definition
type IdxRange struct {
	Low, High int
}

// Partition function
func Partition(collectionLen, partitionSize int) chan IdxRange {
	c := make(chan IdxRange)
	if partitionSize <= 0 {
		close(c)
		return c
	}

	go func() {
		numFullPartitions := collectionLen / partitionSize
		var i int
		for ; i < numFullPartitions; i++ {
			c <- IdxRange{Low: i * partitionSize, High: (i + 1) * partitionSize}
		}

		if collectionLen%partitionSize != 0 {
			c <- IdxRange{Low: i * partitionSize, High: collectionLen}
		}

		close(c)
	}()
	return c
}
