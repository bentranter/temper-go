package temper

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/fnv"
)

const (
	// maxFingerprint is the maxium size of a fingerprint. Using a fingerprint
	// sized 16 bits gives a significantly better false positive rate.
	//
	// (Copied from the Temper backend)
	maxFingerprint = (1 << 16) - 1

	// bucketSize is the number of entries in each bucket.
	//
	// (Copied from the Temper backend)
	bucketSize = 4

	// bytesPerBucket is the number of bytes in a single bucket. The constant
	// 16 is the fingerprint size (uint16), and 8 is the size of a byte
	// (uint8).
	bytesPerBucket = bucketSize * 16 / 8
)

// filterResponse represents the JSON response from the Temper API's public
// filter endpoint.
type filterResponse struct {
	Filter  []byte `json:"filter"`
	Rollout []byte `json:"rollout"`
}

// has computes a 64 bit fnv-1a hash of the given data.
func hash(data []byte) uint64 {
	hash := fnv.New64a()
	hash.Write(data)
	return hash.Sum64()
}

// nextPowerOf2 returns the next power of two.
func nextPowerOf2(n uint64) uint {
	n--
	n |= n >> 1
	n |= n >> 2
	n |= n >> 4
	n |= n >> 8
	n |= n >> 16
	n |= n >> 32
	n++
	return uint(n)
}

// A Bucket contains fingerprints.
type bucket [bucketSize]uint16

// contains returns true if the fingerprint is in the bucket.
func (b *bucket) contains(fingerprint uint16) bool {
	for _, entry := range b {
		if entry == fingerprint {
			return true
		}
	}
	return false
}

type filter struct {
	cap             uint // cap of `Buckets`, used to resize.
	count           uint
	buckets         []bucket // "Height" of the cuckoo filter table.
	bucketIndexMask uint

	rollouts map[uint64]uint8 // feature rollout data outside of filter
}

// from initializes a filter from an encoded byte slice.
func from(fr *filterResponse) (*filter, error) {
	filter := &filter{}

	if fr.Filter != nil {
		if len(fr.Filter)%bucketSize != 0 {
			return nil, errors.New("go-temper: bytes must be a multiple of 4")
		}

		size := len(fr.Filter) / bytesPerBucket
		if size < 1 {
			return nil, errors.New("go-temper: data can not be smaller than 16 (size of a bucket)")
		}

		if nextPowerOf2(uint64(size)) != uint(size) {
			return nil, errors.New("go-temper: size must be a power of 2")
		}

		count := uint(0)
		buckets := make([]bucket, size)
		r := bytes.NewReader(fr.Filter)

		for i, b := range buckets {
			for j := range b {
				if err := binary.Read(r, binary.LittleEndian, &buckets[i][j]); err != nil {
					return nil, fmt.Errorf("go-temper: failed to decode filter from http api response: %w", err)
				}
				if buckets[i][j] != 0 {
					count++
				}
			}
		}

		filter.cap = uint(size)
		filter.buckets = buckets
		filter.count = count
		filter.bucketIndexMask = uint(len(buckets) - 1)
	}

	// Unpack the encoded hashed rollout data.
	rollouts := make(map[uint64]uint8)
	if fr.Rollout != nil {
		r := bytes.NewReader(fr.Rollout)

		entries := make([]uint64, r.Len()/8)
		for i := range entries {
			if err := binary.Read(r, binary.LittleEndian, &entries[i]); err != nil {
				return nil, fmt.Errorf("go-temper: failed to decode rollout data from http api response: %w", err)
			}
		}

		for _, e := range entries {
			high := (e >> 8) << 8
			low := uint8(e & ((1 << 8) - 1))
			rollouts[high] = low
		}

		filter.rollouts = rollouts
	}

	return filter, nil
}

// fingerprintAndIndex returns the fingerprint of the given data, and the
// primary index for insertion.
func (f *filter) fingerprintAndIndex(data []byte) (uint16, uint) {
	// Start by computing the hash of the given data.
	hash := hash(data)

	// Compute the fingerprint.
	shifted := hash >> (64 - 16)
	fingerprint := uint16(shifted%(maxFingerprint-1) + 1)

	// Derive the index using the least significant bits.
	index := uint(hash) & f.bucketIndexMask

	return fingerprint, index
}

// altIndex returns the secondary index to store or retrieve a value in the
// filter.
func (f *filter) altIndex(fingerprint uint16, index uint) uint {
	// Turn the fingerprint into a byte slice so that we can hash it.
	data := make([]byte, 2)
	binary.LittleEndian.PutUint16(data, fingerprint)

	// Compute the hash.
	hash := uint(hash(data))

	// Return the alt index.
	return (index ^ hash) & f.bucketIndexMask
}

// lookupRollout looks up the rollout entry in the filter's rollout table. If
// the value is not found, the returned value is 0, indicating the client (or
// filter or whatever) must consult the filter.
func (f *filter) lookupRollout(data []byte) bool {
	// Compute the hash of the full byte slice in case we need it later.
	hfull := hash(data)

	index := bytes.Index(data, []byte(":"))
	if index > 0 {
		// Fully qualified keys are typically in the format
		// `<feature>:<resource_name>:<actor_id>`, for example, `feature:user:1`.
		// Since the rollout belongs to the top-level feature, we need to trim off
		// everything after the first `:`. If no `:` is present, we use the entire
		// byte slice of data, making the assumption that it is the top-level
		// feature key.
		data = data[:index]
	}

	// Compute the hash of only the feature segment of the byte slice to
	// pull the rollout percentage from the rollouts map.
	hfeat := hash(data)
	high := (hfeat >> 8) << 8
	rollout := f.rollouts[high]

	// Fast path: if the rollout is 100, return true now so we don't have to
	// check the mod of the hash of the entire data byte slice.
	if rollout == 100 {
		return true
	}
	// Otherwise get the last twi digits of the full hash and compare it with
	// the rollout percentage.
	return uint8(hfull%100) <= rollout
}

// lookupFilter checks if the data is in the filter.
func (f *filter) lookupFilter(data []byte) bool {
	if f.buckets == nil {
		return false
	}

	fingerprint, index := f.fingerprintAndIndex(data)
	if f.buckets[index].contains(fingerprint) {
		return true
	}

	index = f.altIndex(fingerprint, index)
	return f.buckets[index].contains(fingerprint)
}

// lookup returns true if data is in the filter or is enabled by the rollout
// data.
func (f *filter) lookup(data []byte) bool {
	if f.lookupRollout(data) {
		return true
	}

	return f.lookupFilter(data)
}
