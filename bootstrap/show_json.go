package main

import (
	"fmt"
	"io/ioutil"
	"time"
)

func showJSONFormat() {
	config := PersistenceConfig{
		Enabled:    true,
		FilePath:   "example.json",
		MaxRetries: 3,
	}

	set := NewHashSetWithPersistence[string](4, config)
	defer set.Close()

	// Add some sample data
	set.Insert("apple")
	set.Insert("banana")
	set.Insert("cherry")
	set.Insert("date")
	set.Insert("elderberry")

	err := set.TriggerSnapshot()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	time.Sleep(100 * time.Millisecond)

	// Read and display the JSON file
	data, err := ioutil.ReadFile("example.json")
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		return
	}

	fmt.Println("📄 JSON Snapshot Format:")
	fmt.Println(string(data))
}
