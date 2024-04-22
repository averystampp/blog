package main

import (
	"log"

	"github.com/averystampp/blog/api"
	"github.com/averystampp/sesame"
	bolt "go.etcd.io/bbolt"
)

func main() {

	rtr := sesame.NewRouter()

	api.StartBlog(rtr)

	db, err := bolt.Open("blog.db", 0660, nil)
	if err != nil {
		log.Fatal(err)
	}

	db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("sessions"))
		if err != nil {
			return err
		}
		_, err = tx.CreateBucketIfNotExists([]byte("posts"))
		if err != nil {
			return err
		}
		_, err = tx.CreateBucketIfNotExists([]byte("users"))

		return err
	})
	api.AddUser()
	defer db.Close()
	rtr.StartServer(":443")
}
