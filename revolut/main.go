package revolut

import (
	"fmt"
	"ibkr-report/broker"
	"strings"
)

func Read(filename string) (*broker.Statement, error) {
	// Revolut does not provide any info within the csv files. Read full path. If revolut is mentioned in the path, continue reading
	// Otherwise, return nil, nil

	if !strings.Contains(filename, "revolut") {
		return nil, broker.ErrNotRecognized
	}

	// This is a placeholder for the actual implementation
	fmt.Println("Reading Revolut statement: ", filename, "... Not implemented.")

	return &broker.Statement{}, nil
}
