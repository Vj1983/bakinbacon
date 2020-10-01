package storage

import (
	"encoding/binary"
	"fmt"
	"time"

	bolt "go.etcd.io/bbolt"
    log "github.com/sirupsen/logrus"
)

const (
	DATABASE_FILE = "goendorse.db"

	BAKING_BUCKET = "bakes"
	NONCE_BUCKET  = "nonces"
)

type Storage struct {
        db *bolt.DB
}

var DB Storage

func init() {

	db, err := bolt.Open(DATABASE_FILE, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		log.Fatal("Failed to init db:", err)
	}
	
	// Ensure some buckets exist, and migrations
	err = db.Update(func(tx *bolt.Tx) error {

		if _, err := tx.CreateBucketIfNotExists([]byte(BAKING_BUCKET)); err != nil {
			return fmt.Errorf("Cannot create baking bucket: %s", err)
		}

		if _, err := tx.CreateBucketIfNotExists([]byte(NONCE_BUCKET)); err != nil {
			return fmt.Errorf("Cannot create nonce bucket: %s", err)
		}

		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	// set variable so main program can access
	DB = Storage{
		db: db,
	}
}

func (s *Storage) Close() {
	s.db.Close()
	log.Info("Database closed")
}

// itob returns an 8-byte big endian representation of v.
func itob(v int) []byte {
    b := make([]byte, 8)
    binary.BigEndian.PutUint64(b, uint64(v))
    return b
}
