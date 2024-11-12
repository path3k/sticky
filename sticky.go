package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"

	_ "github.com/mattn/go-sqlite3"
)

type flags struct {
	add   string
	get   int
	list  bool
	del   int
	purge bool
	width int
}

type Note struct {
	VirtualID int
	ID        int
	Content   string
}

const (
	red    = "\x1b[31m"
	green  = "\x1b[32m"
	yellow = "\x1b[33m"
	blue   = "\x1b[34m"
	reset  = "\x1b[0m"
)

func getDbPath() string {
	dbPath := "./sticky.db"

	if os.Getenv("STICKY_ENV") != "dev" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			log.Fatal(err)
		}

		dbPath = filepath.Join(homeDir, ".local", "share", "sticky", "sticky.db")
		if err := os.MkdirAll(filepath.Dir(dbPath), os.ModePerm); err != nil {
			log.Fatalf("Error creating directory: %v", err)
		}
	}

	return dbPath
}

func initDb() *sql.DB {
	dbPath := getDbPath()

	_, err := os.Stat(dbPath)
	databaseExists := !os.IsNotExist(err)

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatal(err)
		return nil
	}

	stmt := `
	CREATE table IF NOT EXISTS notes (
		id integer NOT NULL PRIMARY KEY,
		note TEXT
	);
	`

	_, err = db.Exec(stmt)
	if err != nil {
		log.Printf("%q: %s\n", err, stmt)
		return nil
	}

	if !databaseExists {
		wd, err := os.Getwd()
		if err != nil {
			log.Println(err)
		}
		// TODO: do some confirmation stuff here like "you are about to create a sticky db at blabla.. do you want to proceed?"
		// doar in caz de dbPath default.. si zici ca you are about to create a DEV db at blabla!!
		fmt.Println(blue + "Created 'sticky.db' database at: " + wd + reset)
	}

	return db
}

func listNotes(db *sql.DB) {
	var rowCount int
	err := db.QueryRow(`SELECT COUNT(*) FROM notes`).Scan(&rowCount)
	if err != nil {
		log.Fatal(err)
	}

	if rowCount != 0 {
		// also ar trebui padding de forma "id + max_row spaces" - "note + max cat width" - "date"
		// hmm.. dar unde pui cap de tabel? perhaps left si pad rest with spaces?
		fmt.Println("count", rowCount)
		fmt.Println("id - note - date maybe")
	}

	stmt, err := db.Prepare(`
		SELECT
			ROW_NUMBER() OVER (ORDER BY id) AS virtual_id,
			note
		FROM notes;
	`)
	if err != nil {
		log.Fatal(err)
	}
	defer stmt.Close()

	rows, err := stmt.Query()
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	for rows.Next() {
		n := new(Note)
		err = rows.Scan(&n.VirtualID, &n.Content)
		if err != nil {
			log.Fatal(err)
		}
		formatNote(*n, len(strconv.Itoa(rowCount)))
	}

	if err = rows.Err(); err != nil {
		log.Fatal(err)
	}
}

func getNoteById(noteId int, db *sql.DB) {
	stmt, err := db.Prepare(`
		WITH ordered_notes AS (
			SELECT
				ROW_NUMBER() OVER (ORDER by id) AS virtual_id,
				note
			FROM notes
		)
		SELECT *
		FROM ordered_notes
		WHERE virtual_id = ?
	`)
	if err != nil {
		log.Fatal(err)
	}
	defer stmt.Close()

	var virtualId int
	var note string
	err = stmt.QueryRow(noteId).Scan(&virtualId, &note)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(virtualId, note)
}

func addNote(content string, db *sql.DB) {
	stmt, err := db.Prepare("INSERT INTO notes(id, note) values(NULL, ?)")
	if err != nil {
		log.Fatal(err)
	}
	defer stmt.Close()

	_, err = stmt.Exec(content)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Successfully added note")
}

func delNotes() {
	str := red + "This operation will delete your entire notes database.\n" + reset +
		"Type \"y\" to proceed, type anything else to cancel.\n" +
		blue + "> " + reset
	fmt.Print(str)

	var answer string
	_, err := fmt.Scan(&answer)
	if err != nil {
		fmt.Println("Error reading input:", err)
		return
	}

	if answer == "y" {
		dbPath := getDbPath()
		os.Remove(dbPath)
		fmt.Println(yellow + "Sticky notes database deleted." + reset)
	} else {
		fmt.Println(green + "Sticky notes database preserved." + reset)
	}
}

func delNote(noteId int, db *sql.DB) {
	stmt, err := db.Prepare(`
		WITH ordered_notes AS (
			SELECT
				id,
				ROW_NUMBER() OVER (ORDER BY id) as virtual_id
			FROM notes
		)
		DELETE FROM notes
		WHERE id = (SELECT id FROM ordered_notes WHERE virtual_id = ?);
	`)
	if err != nil {
		log.Fatal(err)
	}
	defer stmt.Close()

	_, err = stmt.Exec(noteId)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Successfully deleted note #%d\n", noteId)
}

func padString(s string, width int, before int) string {
	ns := ""
	if before != 0 {
		for i := 0; i < before; i++ {
			ns += " "
		}
	}

	ns += s
	for i := len(s); i < width; i++ {
		ns += " "
	}

	return ns
}

//FIXME: the issue is.. formatul e gen "id_-" on the header, "_1-" on note, ca sa
// fie pe acelasi formatting cu id. deci cu toate ca ai id care e len 1, tu
// trebuie sa-l iei ca si cum e len 2 minim.
// cba sa mai lucrez la asta this instant, but gotta think about it.

// NOTE: ah but of course.. daca nu ai --headers, nici macar nu se mai aplica
// regula asta. fun. not impossible to do, just gotta figure out how to sort
// out the params correctly
func formatNote(n Note, width int) {
	sID := strconv.Itoa(n.VirtualID)
	fmt.Printf("%s - %s\n", padString(sID, width, len(sID)), n.Content)
}

//TODO: aight.. e asa.. ai mereu headerul acelasi, dar iti trebuie sa stii ce
// sa contina headerul. unless? nah.. iti trebuie. ca poate vrei --plain at
// some point.
// so.. --plain = doar content, altfel, ai id, content, date. id si content
// default to true, date to false
// asa caaaaa.. un formatHeader si un formatNote? pad probabil la fel peste
// tot, dar tre sa ii dai arguments in functie de ce headers ai.

// si basically si la header si la note le dai ce headers sa foloseasca
// si in functie de aia sa formatezi

func main() {
	f := new(flags)
	flag.StringVar(&f.add, "add", "", "add note")
	flag.IntVar(&f.get, "get", 0, "get note by id")
	flag.BoolVar(&f.list, "list", false, "list all notes")
	flag.IntVar(&f.del, "del", 0, "delete note by id")
	flag.BoolVar(&f.purge, "purge", false, "delete notes database")
	flag.IntVar(&f.width, "width", 0, "limit output to selected number of columns")
	flag.Parse()

	db := initDb()
	if db == nil {
		log.Fatal("Failed to initialize the database")
	}
	defer db.Close()

	switch {
	case f.add != "":
		addNote(f.add, db)
	case f.get != 0:
		getNoteById(f.get, db)
	case f.list:
		listNotes(db)
	case f.del != 0:
		delNote(f.del, db)
	case f.purge:
		delNotes()
	default:
		listNotes(db)
	}
}
