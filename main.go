package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"os"
	"strings"

	// load database driver for sqlite
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	args := os.Args[1:]
	switch len(args) {
	case 1:
		study(args[0])
	case 2:
		switch args[0] {
		case "new":
			neu(args[1])
		case "add":
			add(args[1])
		default:
			usage()
		}
	default:
		usage()
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  word new FILE   create new database")
	fmt.Fprintln(os.Stderr, "  word add FILE   add words to database")
	fmt.Fprintln(os.Stderr, "  word FILE       study")
	os.Exit(1)
}

func check(err error) {
	if err == nil {
		return
	}
	fmt.Fprintln(os.Stderr, "error:", err.Error())
	os.Exit(2)
}

func check1[T any](v T, err error) T {
	check(err)
	return v
}

func read(prompt string) string {
	fmt.Printf("%s => ", prompt)
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		fmt.Println()
		os.Exit(0)
	}
	check(scanner.Err())
	return scanner.Text()
}

func confirm(prompt string, defaultValue bool) bool {
	for {
		if defaultValue {
			prompt = fmt.Sprintf("%s [Yn]?", prompt)
		} else {
			prompt = fmt.Sprintf("%s [yN]?", prompt)
		}
		got := read(prompt)
		switch strings.ToLower(got) {
		case "":
			return defaultValue
		case "y":
			return true
		case "n":
			return false
		}
	}
}

const initDatabaseSQL = `
create table words (
	id    integer primary key,
	front text    not null,
	back  text    not null,
	box   integer not null default 0,
	due   text    not null default (date('now', '+1 day'))
)
`

const addWordSQL = `
insert into words (front, back) values (?, ?)
`

const nextWordSQL = `
select id, box, front, back
from words
where due <= date()
`

const moveWordSQL = `
update words
set box = ?, due = date('now', '+%d days')
where id = ?
`

func neu(dbname string) {
	db := check1(sql.Open("sqlite3", dbname))
	defer db.Close()
	check1(db.Exec(initDatabaseSQL))
}

func add(dbname string) {
	db := check1(sql.Open("sqlite3", dbname))
	defer db.Close()

	front := read("Front")
	back := read("Back")
	check1(db.Exec(addWordSQL, front, back))
}

func study(dbname string) {
	db := check1(sql.Open("sqlite3", dbname))
	defer db.Close()

	for {
		// get the next word
		row := db.QueryRow(nextWordSQL)
		var id, box int
		var front, back string
		err := row.Scan(&id, &box, &front, &back)
		if err == sql.ErrNoRows {
			fmt.Println("Done for today!")
			os.Exit(0)
		}
		check(err)

		// present it to the user
		got := read(front)
		var correct bool
		if got == back {
			fmt.Println("Correct!")
			correct = true
		} else {
			fmt.Println("Wanted:", back)
			correct = confirm("Advance", false)
		}

		// update database
		if correct {
			box++
		} else {
			box = 0
		}
		days := 1 << box
		query := fmt.Sprintf(moveWordSQL, days)
		check1(db.Exec(query, box, id))
	}
}
