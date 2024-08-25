package main

import (
	"bufio"
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

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
		case "export":
			export(args[1])
		case "import":
			imprt(args[1])
		default:
			usage()
		}
	default:
		usage()
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  word new FILE      create new database")
	fmt.Fprintln(os.Stderr, "  word add FILE      add words to database")
	fmt.Fprintln(os.Stderr, "  word export FILE   export database (writes to stdout)")
	fmt.Fprintln(os.Stderr, "  word import FILE   import database (reads from stdout)")
	fmt.Fprintln(os.Stderr, "  word FILE          study")
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

func warn(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format, a...)
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
	if defaultValue {
		prompt = fmt.Sprintf("%s [Yn]?", prompt)
	} else {
		prompt = fmt.Sprintf("%s [yN]?", prompt)
	}
	for {
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
	due   text    not null default (date('now'))
)
`

const addWordSQL = `
insert into words (front, back) values (?, ?)
`

const exportDatabaseSQL = `
select front, back, box, due from words order by id
`

const importRowSQL = `
insert into words (front, back, box, due) values (?, ?, ?, ?)
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

func export(dbname string) {
	db := check1(sql.Open("sqlite3", dbname))
	defer db.Close()

	writer := csv.NewWriter(os.Stdout)
	rows := check1(db.Query(exportDatabaseSQL))
	defer rows.Close()
	for rows.Next() {
		var box int
		var front, back, due string
		check(rows.Scan(&front, &back, &box, &due))
		check(writer.Write([]string{front, back, fmt.Sprint(box), due}))
	}
	writer.Flush()
	check(writer.Error())
}

func imprt(dbname string) {
	db := check1(sql.Open("sqlite3", dbname))
	defer db.Close()

	reader := csv.NewReader(os.Stdin)
	var line int
	for {
		line++
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		check(err)

		if len(record) != 4 {
			warn("invalid record on line %d", line)
			continue
		}
		front := record[0]
		back := record[1]
		box, err := strconv.Atoi(record[2])
		if err != nil {
			warn("invalid record on line %d: not a number: %q", line, record[2])
			continue
		}
		due := record[3]
		_, err = time.Parse(time.DateOnly, due)
		if err != nil {
			warn("invalid record on line %d: invalid date: %q", line, due)
			continue
		}

		check1(db.Exec(importRowSQL, front, back, box, due))
	}
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
		days := 0
		if correct {
			box++
			days = 1 << box
		} else {
			box = 0
		}
		query := fmt.Sprintf(moveWordSQL, days)
		check1(db.Exec(query, box, id))
	}
}
