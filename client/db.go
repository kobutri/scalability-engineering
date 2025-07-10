package main

import (
	"database/sql"
	"fmt"
	"log"
	"shared"
	"time"

	_ "modernc.org/sqlite" // SQLite driver
)

func InitiateDB() {
	db, err := sql.Open("sqlite", "./chats.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Message-Tabelle erstellen
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS Message (
		messageID TEXT PRIMARY KEY,
		timestamp DATETIME NOT NULL,
    	status TEXT NOT NULL CHECK (status IN ('sent', 'delivered', 'received')),
		message TEXT NOT NULL
	)`)
	if err != nil {
		log.Fatal("FAILED TO CREATE MESSAGE TABLE", err)
	}

	// Contact-Tabelle erstellen
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS Contact (
		containerID TEXT, 
		messageID TEXT,
		PRIMARY KEY (containerID, messageID),
		FOREIGN KEY (messageID) REFERENCES Message(messageID)
	)`)
	if err != nil {
		log.Fatal("FAILED TO CREATE CONTACT TABLE", err)
	}
}

// LoadMessagesForClient lädt alle Nachrichten für einen Container (Client)
func LoadMessagesForClient(containerID string) ([]Message, error) {
	db, err := sql.Open("sqlite", "./chats.db")
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT m.messageID, m.timestamp, m.status, m.message
		FROM Message m
		JOIN Contact c ON m.messageID = c.messageID
		WHERE c.containerID = ?
		ORDER BY m.timestamp DESC
	`, containerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var msg Message
		err := rows.Scan(&msg.MessageID, &msg.Timestamp, &msg.Status, &msg.Message)
		if err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}
	return messages, nil
}

func RcvMessage(msg Message) error {
	log.Printf("Speichere Nachricht: %s", msg.Message)
	db, err := sql.Open("sqlite", "./chats.db")
	if err != nil {
		log.Printf("Fehler beim Öffnen der Datenbank: %v", err)
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 1. Überprüfen, ob die Nachricht bereits existiert
	var messageExists int
	err = tx.QueryRow("SELECT COUNT(*) FROM Message WHERE messageID = ?", msg.MessageID).Scan(&messageExists)
	if err != nil {
		return fmt.Errorf("failed to check message existence: %w", err)
	}

	// 2. Nachricht einfügen oder aktualisieren
	if messageExists == 0 {
		_, err = tx.Exec(`
            INSERT INTO Message (messageID, timestamp, status, message)
            VALUES (?, ?, ?, ?)`,
			msg.MessageID, msg.Timestamp, msg.Status, msg.Message)
		if err != nil {
			return fmt.Errorf("failed to insert message: %w", err)
		}
	} else {
		_, err = tx.Exec(`
            UPDATE Message 
            SET timestamp = ?, status = ?, message = ?
            WHERE messageID = ?`,
			msg.Timestamp, msg.Status, msg.Message, msg.MessageID)
		if err != nil {
			return fmt.Errorf("failed to update message: %w", err)
		}
	}

	// 3. Überprüfen, ob der Kontakt bereits existiert
	var contactExists int
	err = tx.QueryRow(`
        SELECT COUNT(*) 
        FROM Contact 
        WHERE containerID = ? AND messageID = ?`,
		msg.ContainerID, msg.MessageID).Scan(&contactExists)
	if err != nil {
		return fmt.Errorf("failed to check contact existence: %w", err)
	}

	// 4. Kontakt einfügen falls nicht vorhanden
	if contactExists == 0 {
		_, err = tx.Exec(`
            INSERT INTO Contact (containerID, messageID)
            VALUES (?, ?)`, // Hier wurden die überflüssigen Parameter entfernt
			msg.ContainerID, msg.MessageID)
		if err != nil {
			return fmt.Errorf("failed to insert contact: %w", err)
		}
	}

	// Transaktion abschließen
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func GetContactsWithLastMessages() ([]ContactWithMessage, error) {
	db, err := sql.Open("sqlite", "./chats.db")
	if err != nil {
		return nil, err
	}
	defer db.Close()

	query := `
        SELECT 
            c.containerID,
            m.message,
            m.timestamp,
            m.status
        FROM Contact c
        JOIN Message m ON c.messageID = m.messageID
        WHERE m.timestamp = (
            SELECT MAX(timestamp) 
            FROM Message m2 
            JOIN Contact c2 ON m2.messageID = c2.messageID 
            WHERE c2.containerID = c.containerID
        )
        ORDER BY m.timestamp DESC
    `

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var contacts []ContactWithMessage
	for rows.Next() {
		var contact ContactWithMessage
		err := rows.Scan(
			&contact.ContainerID,
			&contact.LastMessage,
			&contact.LastMessageTime,
			&contact.LastMessageStatus,
		)
		if err != nil {
			return nil, err
		}
		contacts = append(contacts, contact)
	}

	return contacts, nil
}

type ContactWithMessage struct {
	ContainerID       string
	LastMessage       string
	LastMessageTime   string
	LastMessageStatus string
}

func formatTime(timestamp string) string {
	t, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return timestamp // Fallback zum originalen String
	}
	return t.Format("02.01.2006 15:04")
}

func printAllMessages() error {
	db, err := sql.Open("sqlite", "./chats.db")
	if err != nil {
		return err
	}
	defer db.Close()

	rows, err := db.Query(`SELECT messageID, timestamp, status, message FROM Message ORDER BY timestamp ASC`)
	if err != nil {
		return err
	}
	defer rows.Close()

	fmt.Println("All Messages:")
	for rows.Next() {
		var id, timestamp, status, message string
		err := rows.Scan(&id, &timestamp, &status, &message)
		if err != nil {
			return err
		}
		fmt.Printf("ID: %s | Time: %s | Status: %s | Message: %s\n", id, timestamp, status, message)
	}

	if err = rows.Err(); err != nil {
		return err
	}

	return nil
}

func GetKnownIdentities() ([]shared.ClientIdentity, error) {
	db, err := sql.Open("sqlite", "./chats.db")
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(`SELECT DISTINCT containerID FROM Contact`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []shared.ClientIdentity
	for rows.Next() {
		var containerID string
		if err := rows.Scan(&containerID); err != nil {
			return nil, err
		}
		// Der Name ist leider nicht in der DB – wir setzen ihn auf "" oder "unknown"
		result = append(result, shared.ClientIdentity{
			Name:        "unknown", // oder leer lassen
			ContainerID: containerID,
		})
	}
	return result, nil
}
