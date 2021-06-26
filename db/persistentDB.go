package db

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"path/filepath"
	"time"

	"github.com/boltdb/bolt"
	"github.com/giwty/switch-library-manager/settings"
	"go.uber.org/zap"
)

const (
	DB_FILE_NAME                = "slm.db"
	DB_BUCKET_INTERNAL_METADATA = "internal-metadata"
	DB_FIELD_APP_VERSION        = "app_version"
)

// Persistent library database
type PersistentDB struct {
	db     *bolt.DB
	logger *zap.SugaredLogger
}

// Constructor for the library database
func NewPersistentDB(baseFolder string, l *zap.SugaredLogger, s *settings.AppSettings) *PersistentDB {
	// Instantiate the DB
	db := &PersistentDB{
		logger: l,
	}

	// Check if we need to put the database in the homedir and if we have a homedir
	if s.DBInHomedir && s.Homedir != "" {
		baseFolder = s.GetHomedirPath()
	}

	// Open the database
	db.Open(baseFolder)

	// Update the version
	db.UpdateVersion()

	return db
}

// Open the database
func (pd *PersistentDB) Open(baseFolder string) {
	// Open the database, it will be created if it doesn't exist
	var dbErr error
	pd.db, dbErr = bolt.Open(filepath.Join(baseFolder, DB_FILE_NAME), 0600, &bolt.Options{Timeout: 1 * time.Minute})
	if dbErr != nil {
		// This will exit the app
		pd.logger.Fatal(dbErr)
	}
}

// Close the database
func (pd *PersistentDB) Close() {
	pd.db.Close()
}

// Update the app version
func (pd *PersistentDB) UpdateVersion() {
	// Set the database version to the app version
	pd.db.Update(func(tx *bolt.Tx) error {
		// Create the internal metadata bucket if it does not exist
		bucket := tx.Bucket([]byte(DB_BUCKET_INTERNAL_METADATA))
		if bucket == nil {
			var bucketErr error
			bucket, bucketErr = tx.CreateBucket([]byte(DB_BUCKET_INTERNAL_METADATA))
			if bucket == nil || bucketErr != nil {
				pd.logger.Errorf("create bucket: %s", bucketErr)
				return fmt.Errorf("create bucket: %s", bucketErr)
			}
		}

		// Update the application version
		putErr := bucket.Put([]byte(DB_FIELD_APP_VERSION), []byte(settings.SLM_VERSION))
		if putErr != nil {
			pd.logger.Warnf("failed to save app_version - %v", putErr)
			return putErr
		}

		return nil
	})
}

// Clear a table from the database
func (pd *PersistentDB) ClearTable(tableName string) error {
	err := pd.db.Update(func(tx *bolt.Tx) error {
		err := tx.DeleteBucket([]byte(tableName))
		return err
	})
	return err
}

func (pd *PersistentDB) AddEntry(tableName string, key string, value interface{}) error {
	var err error
	err = pd.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(tableName))
		if b == nil {
			b, err = tx.CreateBucket([]byte(tableName))
			if b == nil || err != nil {
				return fmt.Errorf("create bucket: %s", err)
			}
		}
		var bytesBuff bytes.Buffer
		encoder := gob.NewEncoder(&bytesBuff)
		err := encoder.Encode(value)
		if err != nil {
			return err
		}
		err = b.Put([]byte(key), bytesBuff.Bytes())
		return err
	})
	return err
}

func (pd *PersistentDB) GetEntry(tableName string, key string, value interface{}) error {
	err := pd.db.View(func(tx *bolt.Tx) error {

		b := tx.Bucket([]byte(tableName))
		if b == nil {
			return nil
		}
		v := b.Get([]byte(key))
		if v == nil {
			return nil
		}
		d := gob.NewDecoder(bytes.NewReader(v))

		// Decoding the serialized data
		err := d.Decode(value)
		if err != nil {
			return err
		}
		return nil
	})
	return err
}

/*func (pd *PersistentDB) GetEntries() (map[string]*switchfs.ContentMetaAttributes, error) {
	pd.db.View(func(tx *bolt.Tx) error {
		// Assume bucket exists and has keys
		b := tx.Bucket([]byte(METADATA_TABLENAME))

		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			fmt.Printf("key=%s, value=%s\n", k, v)
		}

		return nil
	})
}*/
