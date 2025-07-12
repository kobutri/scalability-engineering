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
		containerID TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		hostname TEXT
	)`)
	if err != nil {
		log.Fatal("FAILED TO CREATE CONTACT TABLE", err)
	}

	// Message-Tabelle: Nachrichten mit Sender- und Empfänger-IDs
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS Message (
		messageID TEXT PRIMARY KEY,
		timestamp DATETIME NOT NULL,
		status TEXT NOT NULL CHECK (status IN ('sent', 'delivered', 'received')),
		message TEXT NOT NULL,
		senderID TEXT NOT NULL,
		receiverID TEXT NOT NULL
	)`)
	if err != nil {
		log.Fatal("FAILED TO CREATE MESSAGE TABLE", err)
	}
}

// Kontakt hinzufügen
func AddContact(contact Contact) error {
	db, err := sql.Open("sqlite", "./chats.db")
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.Exec(`INSERT OR IGNORE INTO Contact (containerID, name, hostname) VALUES (?, ?, ?)`,
		contact.ContainerID, contact.Name, contact.Hostname)
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
		INSERT OR REPLACE INTO Message (messageID, timestamp, status, message, senderID, receiverID)
		VALUES (?, ?, ?, ?, ?, ?)`,
		msg.MessageID, msg.Timestamp, msg.Status, msg.Message, msg.SenderID, msg.ReceiverID)
	return err
}

// Nachrichtenverlauf mit Kontakt holen (über Container IDs)
func GetChatWithContact(myID, contactID string) ([]Message, error) {
	db, err := sql.Open("sqlite", "./chats.db")
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(`
        SELECT messageID, timestamp, status, message, senderID, receiverID
        FROM Message
        WHERE (senderID = ? AND receiverID = ?)
           OR (senderID = ? AND receiverID = ?)
        ORDER BY timestamp ASC
    `, myID, contactID, contactID, myID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.MessageID, &m.Timestamp, &m.Status, &m.Message, &m.SenderID, &m.ReceiverID); err != nil {
			return nil, err
		}
		messages = append(messages, m)
	}

	for i, msg := range messages {
		log.Printf("Message %d: %s", i+1, msg.Message)
	}
	return messages, nil
}

// Strukturen
type Contact struct {
	ContainerID string
	Name        string
	Hostname    string
}

type Message struct {
	MessageID  string
	Timestamp  string
	Status     string
	Message    string
	SenderID   string
	ReceiverID string
}

// Print prints the Message struct in a readable format.
func (m Message) Print() {
	log.Printf("ID: %s | Time: %s | Status: %s | From: %s | To: %s | Text: %s",
		m.MessageID, m.Timestamp, m.Status, m.SenderID, m.ReceiverID, m.Message)
}
