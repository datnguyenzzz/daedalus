package unpredictable_size

// Motivation: The idea should be similar to the predictable_size byte buffer pool
// But with the unpredictable_size pool, caller don't have to pass the along the predicted size
// while getting the buffer, the internal logic will calibrate the allocated buffer during
// putting back to the pool to avoid the memory fragmentation
