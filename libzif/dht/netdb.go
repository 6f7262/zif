package dht

import "github.com/peterbourgon/diskv"

const (
	BucketSize = 20
)

type NetDB struct {
	table    [][]Address
	addr     Address
	database *diskv.Diskv
}

func NewNetDB(addr Address) *NetDB {
	ret := &NetDB{}
	ret.addr = addr

	// One bucket of addresses per bit in an address
	// At the time of writing, uses roughly 64KB of memory
	ret.table = make([][]Address, AddressBinarySize*8)

	// allocate each bucket
	for n, _ := range ret.table {
		ret.table[n] = make([]Address, 0, BucketSize)
	}

	// setup diskv
	transform := func(s string) []string {
		return []string{}
	}

	ret.database = diskv.New(diskv.Options{
		BasePath:     "data/entries",
		Transform:    transform,
		CacheSizeMax: 10 * 1024 * 1024,
	})

	return ret
}

func (ndb *NetDB) TableLen() int {
	size := 0

	for _, i := range ndb.table {
		size += len(i)
	}

	return size
}

func (ndb *NetDB) Insert(kv *KeyValue) error {
	if !kv.Valid() {
		return &InvalidValue{kv.Key.String()}
	}

	// Find the distance between the kv address and our own address, this is the
	// index in the table
	index := kv.Key.Xor(&ndb.addr).LeadingZeroes()
	bucket := ndb.table[index]

	// there is capacity, insert at the front
	// search to see if it is already inserted

	found := -1

	for n, i := range bucket {
		if i.Equals(&kv.Key) {
			found = n
			break
		}
	}

	// if it already exists, it first needs to be removed from it's old position
	if found != -1 {
		bucket = append(bucket[:found], bucket[found+1:]...)
	} else if len(bucket) == BucketSize {
		// TODO: Ping all peers, remove any inactive
		return &NoCapacity{BucketSize}
	}

	bucket = append([]Address{kv.Key}, bucket...)

	ndb.table[index] = bucket

	// key has been added to the routing table, now store the entry!
	ndb.database.Write(kv.Key.String(), kv.Value)

	return nil
}

// Returns the KeyValue if this node has the address, nil otherwise.
func (ndb *NetDB) Query(addr Address) (*KeyValue, error) {
	if !ndb.database.Has(addr.String()) {
		return nil, nil
	}

	value, err := ndb.database.Read(addr.String())

	if err != nil {
		return nil, err
	}

	return NewKeyValue(addr, value), nil
}
