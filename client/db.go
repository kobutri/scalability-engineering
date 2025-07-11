package main

import (
	"database/sql"
	"log"

	_ "modernc.org/sqlite"
)

// Neue Tabellenstruktur
func InitiateDB() {
	db, err := sql.Open("sqlite", "./chats.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS Contact (
		name TEXT PRIMARY KEY
	)`)
	if err != nil {
		log.Fatal("FAILED TO CREATE CONTACT TABLE", err)
	}

	// Message-Tabelle: Nachrichten mit Sender- und Empfänger-Namen
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS Message (
		messageID TEXT PRIMARY KEY,
		timestamp DATETIME NOT NULL,
		status TEXT NOT NULL CHECK (status IN ('sent', 'delivered', 'received')),
		message TEXT NOT NULL,
		senderName TEXT NOT NULL,
		receiverName TEXT NOT NULL
	)`)
	if err != nil {
		log.Fatal("FAILED TO CREATE MESSAGE TABLE", err)
	}
}

// Kontakt hinzufügen
func AddContact(name string) error {
	db, err := sql.Open("sqlite", "./chats.db")
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.Exec(`INSERT OR IGNORE INTO Contact (name) VALUES (?)`, name)
	return err
}

// Nachricht speichern
func RcvMessage(msg Message) error {
	db, err := sql.Open("sqlite", "./chats.db")
	if err != nil {
		return err
	}
	defer db.Close()
	log.Println("Speichere Nachricht:", msg)
	_, err = db.Exec(`
		INSERT OR REPLACE INTO Message (messageID, timestamp, status, message, senderName, receiverName)
		VALUES (?, ?, ?, ?, ?, ?)`,
		msg.MessageID, msg.Timestamp, msg.Status, msg.Message, msg.SenderName, msg.ReceiverName)
	return err
}

// Nachrichtenverlauf mit Kontakt holen (über Namen)
func GetChatWithContact(myName, contactName string) ([]Message, error) {
	db, err := sql.Open("sqlite", "./chats.db")
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(`
        SELECT messageID, timestamp, status, message, senderName, receiverName
        FROM Message
        WHERE (senderName = ? AND receiverName = ?)
           OR (senderName = ? AND receiverName = ?)
        ORDER BY timestamp ASC
    `, myName, contactName, contactName, myName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.MessageID, &m.Timestamp, &m.Status, &m.Message, &m.SenderName, &m.ReceiverName); err != nil {
			return nil, err
		}
		messages = append(messages, m)
	}

	log.Printf("Found %d messages for contact %s", len(messages), contactName)
	for i, msg := range messages {
		log.Printf("Message %d: %s", i+1, msg.Message)
	}
	return messages, nil
}

// Strukturen
type Contact struct {
	Name string
}

type Message struct {
	MessageID    string
	Timestamp    string
	Status       string
	Message      string
	SenderName   string
	ReceiverName string
}

// Print prints the Message struct in a readable format.
func (m Message) Print() {
	log.Printf("ID: %s | Time: %s | Status: %s | From: %s | To: %s | Text: %s",
		m.MessageID, m.Timestamp, m.Status, m.SenderName, m.ReceiverName, m.Message)
}
