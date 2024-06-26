package api

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/averystampp/sesame"
	"github.com/google/uuid"
	bolt "go.etcd.io/bbolt"
	"golang.org/x/crypto/bcrypt"
)

type Post struct {
	Id    int
	Title string
	Date  string
	Body  template.HTML
	Draft bool
}

func StartBlog(rtr *sesame.Router) {
	// api
	rtr.Post("/post/create", Protected(CreatePost))
	rtr.Post("/login", Login)
	rtr.Get("/delete/{id}", Protected(Delete))
	rtr.Post("/post/update/{id}", Protected(Update))
	// views
	rtr.Get("/post/{id}", PostByID)
	rtr.Get("/", Index)
	rtr.Get("/login", LoginPage)
	rtr.Get("/editor", Protected(Editor))
	rtr.Get("/dashboard", Protected(Dashboard))
	rtr.Get("/edit/{id}", Protected(Edit))
}

func CreatePost(ctx sesame.Context) error {
	post := ctx.Request().PostFormValue("post")
	if post == "" {
		return fmt.Errorf("must have post text")
	}
	title := ctx.Request().PostFormValue("title")
	if title == "" {
		return fmt.Errorf("must have post title")
	}

	draft := ctx.Request().PostFormValue("draft")

	db, err := bolt.Open("./db/blog.db", 0600, nil)
	if err != nil {
		return err
	}
	defer db.Close()
	return db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte("posts"))
		if err != nil {
			return err
		}
		id, err := bucket.NextSequence()
		if err != nil {
			return err
		}

		y, m, d := time.Now().Date()
		post := Post{
			Id:    int(id),
			Title: title,
			Date:  fmt.Sprintf("%s %d, %d", m.String(), d, y),
			Body:  template.HTML(post),
		}
		if draft != "" {
			post.Draft = true
		}

		asJson, err := json.Marshal(&post)
		if err != nil {
			return err
		}

		err = bucket.Put([]byte(strconv.FormatUint(id, 10)), []byte(asJson))
		if err != nil {
			return err
		}
		http.Redirect(ctx.Response(), ctx.Request(), "/", http.StatusSeeOther)
		return nil
	})
}

func Index(ctx sesame.Context) error {
	db, err := bolt.Open("./db/blog.db", 0660, nil)
	if err != nil {
		return err
	}
	defer db.Close()
	var posts []Post
	err = db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("posts"))
		return bucket.ForEach(func(k, v []byte) error {
			var post Post
			err := json.Unmarshal(v, &post)
			if err != nil {
				return err
			}
			if !post.Draft {
				posts = append(posts, post)
			}
			return nil
		})
	})
	if err != nil {
		return err
	}
	tmpl, err := template.ParseFiles("./pages/index.html")
	if err != nil {
		return err
	}
	return tmpl.Execute(ctx.Response(), posts)
}

func PostByID(ctx sesame.Context) error {
	id := ctx.Request().PathValue("id")
	if id == "" {
		return fmt.Errorf("must have post id in params")
	}
	db, err := bolt.Open("./db/blog.db", 0660, nil)
	if err != nil {
		return err
	}
	defer db.Close()
	return db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("posts"))
		body := bucket.Get([]byte(id))
		if body == nil {
			return fmt.Errorf("no matching id")
		}
		var post Post
		err := json.Unmarshal(body, &post)
		if err != nil {
			return err
		}
		ctx.Response().Header().Set("Content-Type", "text/html")

		tmpl, err := template.ParseFiles("./pages/test.html")
		if err != nil {
			return err
		}

		return tmpl.Execute(ctx.Response(), post)
	})
}

func Editor(ctx sesame.Context) error {
	tmpl, err := template.ParseFiles("./pages/editor.html")
	if err != nil {
		return err
	}

	return tmpl.Execute(ctx.Response(), nil)
}

func Login(ctx sesame.Context) error {
	username := ctx.Request().PostFormValue("username")
	if username == "" {
		return fmt.Errorf("must have username to login")
	}
	password := ctx.Request().PostFormValue("password")
	if password == "" {
		return fmt.Errorf("must have password to login")
	}
	db, err := bolt.Open("./db/blog.db", 0660, nil)
	if err != nil {
		return err
	}
	defer db.Close()

	err = db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("users"))
		pwrd := bucket.Get([]byte(username))

		return bcrypt.CompareHashAndPassword(pwrd, []byte(password))
	})
	if err != nil {
		return fmt.Errorf("password is incorrect")
	}

	uuid := uuid.New()
	expires := time.Now().Add(time.Hour * 24)
	http.SetCookie(ctx.Response(), &http.Cookie{
		Name:    "session",
		Value:   uuid.String(),
		Expires: expires,
	})

	err = db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("sessions"))
		bucket.Put([]byte(uuid.String()), []byte(expires.Format(time.DateTime)))
		return nil
	})
	if err != nil {
		return err
	}
	http.Redirect(ctx.Response(), ctx.Request(), "/", http.StatusSeeOther)
	return nil
}

func Protected(h sesame.Handler) sesame.Handler {
	return func(ctx sesame.Context) error {
		session, err := ctx.Request().Cookie("session")
		if err != nil {
			return err
		}
		if err = session.Valid(); err != nil {
			return err
		}
		db, err := bolt.Open("./db/blog.db", 0660, nil)
		if err != nil {
			db.Close()
			return err
		}
		err = db.View(func(tx *bolt.Tx) error {
			bucket := tx.Bucket([]byte("sessions"))
			exp := bucket.Get([]byte(session.Value))
			t, err := time.Parse(time.DateTime, string(exp))
			if err != nil {
				db.Close()
				return err
			}
			if t.Before(time.Now()) {
				db.Close()
				return fmt.Errorf("token is expired")
			}
			return nil
		})
		if err != nil {
			db.Close()
			return err
		}
		db.Close()
		if err = h(ctx); err != nil {
			return err
		}
		return nil
	}
}

func AddUser() {
	username, ok := os.LookupEnv("username")
	if !ok {
		fmt.Println("Did not create user")
		return
	}
	password, ok := os.LookupEnv("password")
	if !ok {
		fmt.Println("Did not create user")
		return
	}
	db, err := bolt.Open("./db/blog.db", 0660, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	err = db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("users"))
		hsh, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		return bucket.Put([]byte(username), hsh)
	})
	if err != nil {
		log.Fatal(err)
	}
}

func LoginPage(ctx sesame.Context) error {
	tmpl, err := template.ParseFiles("./pages/login.html")
	if err != nil {
		return err
	}
	return tmpl.Execute(ctx.Response(), nil)
}

func Delete(ctx sesame.Context) error {
	id := ctx.Request().PathValue("id")
	if id == "" {
		return fmt.Errorf("must have id in params")
	}
	db, err := bolt.Open("./db/blog.db", 0660, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	err = db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte("posts")).Delete([]byte(id))
	})
	if err != nil {
		return err
	}
	http.Redirect(ctx.Response(), ctx.Request(), "/dashboard", http.StatusSeeOther)
	return nil
}

func Edit(ctx sesame.Context) error {
	id := ctx.Request().PathValue("id")
	if id == "" {
		return fmt.Errorf("must have id in params")
	}
	db, err := bolt.Open("./db/blog.db", 0660, nil)
	if err != nil {
		return err
	}
	defer db.Close()
	var post Post
	err = db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("posts"))
		body := bucket.Get([]byte(id))
		err := json.Unmarshal(body, &post)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	tmpl, err := template.ParseFiles("./pages/edit.html")
	if err != nil {
		return err
	}
	return tmpl.Execute(ctx.Response(), post)
}

func Update(ctx sesame.Context) error {
	post := ctx.Request().PostFormValue("post")
	if post == "" {
		return fmt.Errorf("must have post text")
	}
	title := ctx.Request().PostFormValue("title")
	if title == "" {
		return fmt.Errorf("must have post title")
	}
	draft := ctx.Request().PostFormValue("draft")
	id := ctx.Request().PathValue("id")

	idasint, err := strconv.Atoi(id)
	if err != nil {
		return err
	}
	db, err := bolt.Open("./db/blog.db", 0600, nil)
	if err != nil {
		return err
	}
	defer db.Close()
	return db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte("posts"))
		if err != nil {
			return err
		}

		y, m, d := time.Now().Date()
		post := Post{
			Id:    idasint,
			Title: title,
			Date:  fmt.Sprintf("%s %d, %d", m.String(), d, y),
			Body:  template.HTML(post),
		}
		if draft != "" {
			post.Draft = true
		}
		asJson, err := json.Marshal(&post)
		if err != nil {
			return err
		}
		err = bucket.Put([]byte(id), []byte(asJson))
		if err != nil {
			return err
		}
		http.Redirect(ctx.Response(), ctx.Request(), "/", http.StatusSeeOther)
		return nil
	})
}

func Dashboard(ctx sesame.Context) error {
	db, err := bolt.Open("./db/blog.db", 0660, nil)
	if err != nil {
		return err
	}
	defer db.Close()
	var posts []Post
	err = db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("posts"))
		return bucket.ForEach(func(k, v []byte) error {
			var post Post
			err := json.Unmarshal(v, &post)
			if err != nil {
				return err
			}
			posts = append(posts, post)
			return nil

		})
	})
	if err != nil {
		return err
	}
	tmpl, err := template.ParseFiles("./pages/dash.html")
	if err != nil {
		return err
	}
	return tmpl.Execute(ctx.Response(), posts)
}
