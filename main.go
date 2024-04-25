package main

import (
	"fmt"
	"log"
	"os"

	"github.com/averystampp/blog/api"
	"github.com/averystampp/sesame"
	bolt "go.etcd.io/bbolt"
)

func main() {

	rtr := sesame.NewRouter()

	api.StartBlog(rtr)

	db, err := bolt.Open("./db/blog.db", 0660, nil)
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
	db.Close()
	api.AddUser()
	if os.Getenv("prod") == "true" {
		fmt.Println("starting with tls")
		rtr.CertFile = "ssl.pem"
		rtr.KeyFile = "ssl.key"
		rtr.StartServerTLS(":8080")
	}

	fmt.Println("starting on 8080")
	rtr.StartServer(":8080")
}
